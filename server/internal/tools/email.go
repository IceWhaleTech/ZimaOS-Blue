package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

// EmailMessage represents an inbox message managed by the local email tool and
// daily summary aggregation.
type EmailMessage struct {
	ID          string    `json:"id"`
	ThreadID    string    `json:"thread_id,omitempty"`
	Subject     string    `json:"subject"`
	SenderName  string    `json:"sender_name,omitempty"`
	SenderEmail string    `json:"sender_email"`
	Recipients  []string  `json:"recipients,omitempty"`
	Snippet     string    `json:"snippet,omitempty"`
	Body        string    `json:"body,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	Priority    string    `json:"priority,omitempty"`
	Unread      bool      `json:"unread"`
	Archived    bool      `json:"archived"`
	ReceivedAt  time.Time `json:"received_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EmailQueryOptions describes list/search/filter constraints.
type EmailQueryOptions struct {
	Query    string
	From     string
	Label    string
	Priority string
	Unread   *bool
	Archived *bool
	Limit    int
}

// EmailSummary captures a concise inbox overview.
type EmailSummary struct {
	Total             int            `json:"total"`
	Unread            int            `json:"unread"`
	Archived          int            `json:"archived"`
	HighPriority      int            `json:"high_priority"`
	ActionableCount   int            `json:"actionable_count"`
	LabelCounts       map[string]int `json:"label_counts,omitempty"`
	TopActionItems    []string       `json:"top_action_items,omitempty"`
	HighlightMessages []EmailMessage `json:"highlight_messages,omitempty"`
}

// EmailService is the backing store used by the email tool and daily summary.
type EmailService interface {
	List(ctx context.Context, ownerID string, opts EmailQueryOptions) ([]EmailMessage, error)
	Get(ctx context.Context, ownerID, id string) (*EmailMessage, error)
	Archive(ctx context.Context, ownerID, id string, archived bool) (*EmailMessage, error)
	Label(ctx context.Context, ownerID, id string, add, remove []string) (*EmailMessage, error)
	Summarize(ctx context.Context, ownerID string, opts EmailQueryOptions) (*EmailSummary, error)
}

// LocalEmailService persists inbox messages in SQLite.
type LocalEmailService struct {
	db     *sql.DB
	readDB *sql.DB
	now    func() time.Time
}

// NewLocalEmailService creates the email table and returns a local inbox service.
func NewLocalEmailService(db *sql.DB) (*LocalEmailService, error) {
	return NewLocalEmailServiceWithReadDB(db, db)
}

// NewLocalEmailServiceWithReadDB creates the email table and returns a local
// inbox service with separate write and read database handles.
func NewLocalEmailServiceWithReadDB(writeDB, readDB *sql.DB) (*LocalEmailService, error) {
	if writeDB == nil {
		return nil, errors.New("email database is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	if _, err := writeDB.Exec(`CREATE TABLE IF NOT EXISTS tool_email_messages (
		id            TEXT PRIMARY KEY,
		owner_id      TEXT NOT NULL,
		thread_id     TEXT NOT NULL DEFAULT '',
		subject       TEXT NOT NULL,
		sender_name   TEXT NOT NULL DEFAULT '',
		sender_email  TEXT NOT NULL DEFAULT '',
		recipients    TEXT NOT NULL DEFAULT '[]',
		snippet       TEXT NOT NULL DEFAULT '',
		body          TEXT NOT NULL DEFAULT '',
		labels        TEXT NOT NULL DEFAULT '[]',
		priority      TEXT NOT NULL DEFAULT 'normal',
		unread        INTEGER NOT NULL DEFAULT 1,
		archived      INTEGER NOT NULL DEFAULT 0,
		received_at   TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	)`); err != nil {
		return nil, err
	}
	if _, err := writeDB.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_email_owner_received ON tool_email_messages(owner_id, received_at DESC)`); err != nil {
		return nil, err
	}
	return &LocalEmailService{
		db:     writeDB,
		readDB: readDB,
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *LocalEmailService) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *LocalEmailService) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "tool_email_messages")
}

func (s *LocalEmailService) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "tool_email_messages")
}

type emailMessageRow struct {
	ID          string `json:"id" zorm:"id"`
	ThreadID    string `json:"thread_id" zorm:"thread_id"`
	Subject     string `json:"subject" zorm:"subject"`
	SenderName  string `json:"sender_name" zorm:"sender_name"`
	SenderEmail string `json:"sender_email" zorm:"sender_email"`
	Recipients  string `json:"recipients" zorm:"recipients"`
	Snippet     string `json:"snippet" zorm:"snippet"`
	Body        string `json:"body" zorm:"body"`
	Labels      string `json:"labels" zorm:"labels"`
	Priority    string `json:"priority" zorm:"priority"`
	Unread      int    `json:"unread" zorm:"unread"`
	Archived    int    `json:"archived" zorm:"archived"`
	ReceivedAt  string `json:"received_at" zorm:"received_at"`
	UpdatedAt   string `json:"updated_at" zorm:"updated_at"`
}

// SetNowFunc overrides the clock, mainly for tests.
func (s *LocalEmailService) SetNowFunc(fn func() time.Time) {
	if s == nil || fn == nil {
		return
	}
	s.now = fn
}

// SeedFixtures inserts an explicit fixture dataset for an owner.
func (s *LocalEmailService) SeedFixtures(ctx context.Context, ownerID string, fixtures []EmailMessage) error {
	if s == nil {
		return errors.New("email service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	messageTable := z.TableContext(ctx, tx, "tool_email_messages")
	if _, err := messageTable.Delete(z.Where(z.Eq("owner_id", ownerID))); err != nil {
		return err
	}
	for _, msg := range fixtures {
		if err := insertEmailMessage(ctx, tx, ownerID, msg); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertEmailMessage(ctx context.Context, tx *sql.Tx, ownerID string, msg EmailMessage) error {
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = uuid.NewString()
	}
	if strings.TrimSpace(msg.ThreadID) == "" {
		msg.ThreadID = msg.ID
	}
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = time.Now().UTC()
	}
	if msg.UpdatedAt.IsZero() {
		msg.UpdatedAt = msg.ReceivedAt
	}
	if strings.TrimSpace(msg.Priority) == "" {
		msg.Priority = "normal"
	}
	recipientsJSON, err := json.Marshal(normalizeStringList(msg.Recipients))
	if err != nil {
		return err
	}
	labelsJSON, err := json.Marshal(normalizeStringList(msg.Labels))
	if err != nil {
		return err
	}
	_, err = z.TableContext(ctx, tx, "tool_email_messages").Insert(z.V{
		"id":           msg.ID,
		"owner_id":     ownerID,
		"thread_id":    strings.TrimSpace(msg.ThreadID),
		"subject":      strings.TrimSpace(msg.Subject),
		"sender_name":  strings.TrimSpace(msg.SenderName),
		"sender_email": strings.TrimSpace(msg.SenderEmail),
		"recipients":   string(recipientsJSON),
		"snippet":      strings.TrimSpace(msg.Snippet),
		"body":         strings.TrimSpace(msg.Body),
		"labels":       string(labelsJSON),
		"priority":     normalizeEmailPriority(msg.Priority),
		"unread":       boolToInt(msg.Unread),
		"archived":     boolToInt(msg.Archived),
		"received_at":  msg.ReceivedAt.UTC().Format(time.RFC3339),
		"updated_at":   msg.UpdatedAt.UTC().Format(time.RFC3339),
	})
	return err
}

// List returns messages for a user after applying search/filter options.
func (s *LocalEmailService) List(ctx context.Context, ownerID string, opts EmailQueryOptions) ([]EmailMessage, error) {
	if s == nil {
		return nil, errors.New("email service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	var rows []emailMessageRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("owner_id", ownerID)),
		z.OrderBy("received_at DESC"),
	)
	if err != nil {
		return nil, err
	}

	result := make([]EmailMessage, 0, 8)
	for i := range rows {
		msg := rowToEmailMessage(rows[i])
		if !emailMatchesQuery(msg, opts) {
			continue
		}
		result = append(result, msg)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Unread != result[j].Unread {
			return result[i].Unread
		}
		if emailPriorityRank(result[i].Priority) != emailPriorityRank(result[j].Priority) {
			return emailPriorityRank(result[i].Priority) > emailPriorityRank(result[j].Priority)
		}
		return result[i].ReceivedAt.After(result[j].ReceivedAt)
	})
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// Get retrieves a single message by ID.
func (s *LocalEmailService) Get(ctx context.Context, ownerID, id string) (*EmailMessage, error) {
	if s == nil {
		return nil, errors.New("email service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	var rows []emailMessageRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("owner_id", ownerID), z.Eq("id", strings.TrimSpace(id))),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	msg := rowToEmailMessage(rows[0])
	return &msg, nil
}

// Archive toggles the archived flag for a message.
func (s *LocalEmailService) Archive(ctx context.Context, ownerID, id string, archived bool) (*EmailMessage, error) {
	if s == nil {
		return nil, errors.New("email service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	updatedAt := s.now().UTC().Format(time.RFC3339)
	affected, err := s.table(ctx).Update(
		z.V{
			"archived":   boolToInt(archived),
			"updated_at": updatedAt,
		},
		z.Fields("archived", "updated_at"),
		z.Where(z.Eq("owner_id", ownerID), z.Eq("id", strings.TrimSpace(id))),
	)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, nil
	}
	return s.Get(ctx, ownerID, id)
}

// Label adds/removes labels for a message and returns the updated row.
func (s *LocalEmailService) Label(ctx context.Context, ownerID, id string, add, remove []string) (*EmailMessage, error) {
	if s == nil {
		return nil, errors.New("email service not configured")
	}
	current, err := s.Get(ctx, ownerID, id)
	if err != nil || current == nil {
		return current, err
	}
	nextLabels := mergeLabels(current.Labels, add, remove)
	labelsJSON, err := json.Marshal(nextLabels)
	if err != nil {
		return nil, err
	}
	_, err = s.table(ctx).Update(
		z.V{
			"labels":     string(labelsJSON),
			"updated_at": s.now().UTC().Format(time.RFC3339),
		},
		z.Fields("labels", "updated_at"),
		z.Where(z.Eq("owner_id", normalizeProductivityOwnerID(ownerID)), z.Eq("id", strings.TrimSpace(id))),
	)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, ownerID, id)
}

// Summarize returns a compact overview of the inbox slice described by opts.
func (s *LocalEmailService) Summarize(ctx context.Context, ownerID string, opts EmailQueryOptions) (*EmailSummary, error) {
	if s == nil {
		return nil, errors.New("email service not configured")
	}
	messages, err := s.List(ctx, ownerID, EmailQueryOptions{
		Query:    opts.Query,
		From:     opts.From,
		Label:    opts.Label,
		Priority: opts.Priority,
		Unread:   opts.Unread,
		Archived: opts.Archived,
		Limit:    50,
	})
	if err != nil {
		return nil, err
	}
	summary := &EmailSummary{
		Total:       len(messages),
		LabelCounts: make(map[string]int),
	}
	type scoredMessage struct {
		msg   EmailMessage
		score int
	}
	scored := make([]scoredMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Unread {
			summary.Unread++
		}
		if msg.Archived {
			summary.Archived++
		}
		if emailPriorityRank(msg.Priority) >= emailPriorityRank("high") {
			summary.HighPriority++
		}
		score := emailActionScore(msg)
		if score > 0 {
			summary.ActionableCount++
			scored = append(scored, scoredMessage{msg: msg, score: score})
		}
		for _, label := range normalizeStringList(msg.Labels) {
			summary.LabelCounts[label]++
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].msg.ReceivedAt.After(scored[j].msg.ReceivedAt)
	})
	limit := len(scored)
	if limit > 3 {
		limit = 3
	}
	for i := 0; i < limit; i++ {
		msg := scored[i].msg
		summary.HighlightMessages = append(summary.HighlightMessages, msg)
		summary.TopActionItems = append(summary.TopActionItems, formatEmailHighlight(msg))
	}
	return summary, nil
}

type emailScanner interface {
	Scan(dest ...interface{}) error
}

func rowToEmailMessage(row emailMessageRow) EmailMessage {
	msg := EmailMessage{
		ID:          row.ID,
		ThreadID:    row.ThreadID,
		Subject:     row.Subject,
		SenderName:  row.SenderName,
		SenderEmail: row.SenderEmail,
		Snippet:     row.Snippet,
		Body:        row.Body,
		Unread:      row.Unread != 0,
		Archived:    row.Archived != 0,
		Priority:    normalizeEmailPriority(row.Priority),
	}
	_ = json.Unmarshal([]byte(row.Recipients), &msg.Recipients)
	_ = json.Unmarshal([]byte(row.Labels), &msg.Labels)
	msg.ReceivedAt, _ = time.Parse(time.RFC3339, row.ReceivedAt)
	msg.UpdatedAt, _ = time.Parse(time.RFC3339, row.UpdatedAt)
	msg.Labels = normalizeStringList(msg.Labels)
	msg.Recipients = normalizeStringList(msg.Recipients)
	return msg
}

func scanEmailMessage(scanner emailScanner) (EmailMessage, error) {
	var msg EmailMessage
	var recipientsJSON string
	var labelsJSON string
	var unreadInt int
	var archivedInt int
	var receivedAt string
	var updatedAt string
	err := scanner.Scan(
		&msg.ID,
		&msg.ThreadID,
		&msg.Subject,
		&msg.SenderName,
		&msg.SenderEmail,
		&recipientsJSON,
		&msg.Snippet,
		&msg.Body,
		&labelsJSON,
		&msg.Priority,
		&unreadInt,
		&archivedInt,
		&receivedAt,
		&updatedAt,
	)
	if err != nil {
		return EmailMessage{}, err
	}
	_ = json.Unmarshal([]byte(recipientsJSON), &msg.Recipients)
	_ = json.Unmarshal([]byte(labelsJSON), &msg.Labels)
	msg.Unread = unreadInt != 0
	msg.Archived = archivedInt != 0
	msg.Priority = normalizeEmailPriority(msg.Priority)
	msg.ReceivedAt, _ = time.Parse(time.RFC3339, receivedAt)
	msg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	msg.Labels = normalizeStringList(msg.Labels)
	msg.Recipients = normalizeStringList(msg.Recipients)
	return msg, nil
}

func emailMatchesQuery(msg EmailMessage, opts EmailQueryOptions) bool {
	if opts.Unread != nil && msg.Unread != *opts.Unread {
		return false
	}
	if opts.Archived != nil && msg.Archived != *opts.Archived {
		return false
	}
	if rawPriority := strings.TrimSpace(opts.Priority); rawPriority != "" {
		if priority := normalizeEmailPriority(rawPriority); priority != "any" && priority != msg.Priority {
			return false
		}
	}
	if from := strings.ToLower(strings.TrimSpace(opts.From)); from != "" {
		candidate := strings.ToLower(strings.TrimSpace(msg.SenderName + " " + msg.SenderEmail))
		if !strings.Contains(candidate, from) {
			return false
		}
	}
	if label := strings.ToLower(strings.TrimSpace(opts.Label)); label != "" {
		found := false
		for _, item := range msg.Labels {
			if strings.EqualFold(strings.TrimSpace(item), label) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(opts.Query)); query != "" {
		haystack := strings.ToLower(strings.Join([]string{
			msg.Subject,
			msg.SenderName,
			msg.SenderEmail,
			msg.Snippet,
			msg.Body,
			strings.Join(msg.Labels, " "),
		}, " "))
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	return true
}

func normalizeEmailPriority(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "normal", "medium":
		return "normal"
	case "high", "urgent", "critical":
		return "high"
	case "low":
		return "low"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func emailPriorityRank(priority string) int {
	switch normalizeEmailPriority(priority) {
	case "high":
		return 3
	case "normal":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func emailActionScore(msg EmailMessage) int {
	score := 0
	if msg.Unread {
		score += 2
	}
	score += emailPriorityRank(msg.Priority) * 2
	lower := strings.ToLower(strings.Join([]string{msg.Subject, msg.Snippet, msg.Body}, " "))
	cues := []string{"action required", "approve", "deadline", "today", "urgent", "before", "review"}
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			score += 2
		}
	}
	return score
}

func formatEmailHighlight(msg EmailMessage) string {
	subject := strings.TrimSpace(msg.Subject)
	if subject == "" {
		subject = "Untitled email"
	}
	sender := strings.TrimSpace(msg.SenderName)
	if sender == "" {
		sender = strings.TrimSpace(msg.SenderEmail)
	}
	if sender == "" {
		return subject
	}
	return fmt.Sprintf("%s — %s", subject, sender)
}

// EmailTool provides inbox triage actions over an extensible email service.
type EmailTool struct {
	service EmailService
}

// NewEmailTool creates a native inbox management tool.
func NewEmailTool(service EmailService) *EmailTool {
	return &EmailTool{service: service}
}

// Definition returns the tool schema.
func (t *EmailTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "email",
		Description: "Manage inbox records from the configured email backend. List, get, search, filter, archive, label, or summarize emails for inbox triage, sender lookups, urgent mail review, and daily summary workflows.",
		Icon:        "email",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"list", "get", "search", "filter", "archive", "label", "summarize"},
					"description": "Email action. Defaults to list, get, or search based on provided arguments.",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Message ID for get/archive/label.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Free-text search across subject, sender, snippet, and body.",
				},
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Sender name or email filter.",
				},
				"label": map[string]interface{}{
					"type":        "string",
					"description": "Single label filter or label to add.",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Labels to add for action=label.",
				},
				"remove_labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Labels to remove for action=label.",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"high", "normal", "low"},
					"description": "Priority filter.",
				},
				"unread": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to restrict to unread mail.",
				},
				"archived": map[string]interface{}{
					"type":        "boolean",
					"description": "Archived filter, or target archive state for action=archive (default true).",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum messages to return (default 10).",
				},
				"include_body": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include full body content in list/search results.",
				},
			},
		},
	}
}

// Execute runs the requested inbox operation.
func (t *EmailTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("email service not available")
	}
	action := normalizeEmailAction(firstCompatString(args, "action", "op", "operation", "command"), args)
	ownerID := normalizeProductivityOwnerID(GetUserID(ctx))
	lang := GetLang(ctx)
	switch action {
	case "list", "search", "filter":
		opts := emailQueryOptionsFromArgs(args)
		messages, err := t.service.List(ctx, ownerID, opts)
		if err != nil {
			return nil, err
		}
		includeBody := compatArgBool(args, "include_body", "includeBody")
		resultMessages := make([]map[string]interface{}, 0, len(messages))
		for _, msg := range messages {
			resultMessages = append(resultMessages, emailMessageForResult(msg, includeBody))
		}
		message := emailLocalized(
			lang,
			fmt.Sprintf("Inbox results: %d message(s) matched.", len(resultMessages)),
			fmt.Sprintf("收件箱结果：匹配到 %d 封邮件。", len(resultMessages)),
		)
		return map[string]interface{}{
			"status":  "success",
			"message": message,
			"count":   len(resultMessages),
			"emails":  resultMessages,
			"query":   strings.TrimSpace(opts.Query),
		}, nil
	case "get":
		id := firstCompatString(args, "id", "email_id", "emailId", "message_id", "messageId")
		if id == "" {
			return nil, errors.New("id is required")
		}
		msg, err := t.service.Get(ctx, ownerID, id)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, errors.New("email not found")
		}
		return map[string]interface{}{
			"status":  "success",
			"message": emailLocalized(lang, fmt.Sprintf("Opened email: %s", msg.Subject), fmt.Sprintf("已打开邮件：%s", msg.Subject)),
			"email":   emailMessageForResult(*msg, true),
		}, nil
	case "archive":
		id := firstCompatString(args, "id", "email_id", "emailId", "message_id", "messageId")
		if id == "" {
			return nil, errors.New("id is required")
		}
		archived := true
		if raw, ok := compatArgValue(args, "archived"); ok {
			archived = compatBoolValue(raw, true)
		}
		msg, err := t.service.Archive(ctx, ownerID, id, archived)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, errors.New("email not found")
		}
		verbEn := "Archived"
		verbZh := "已归档"
		if !archived {
			verbEn = "Unarchived"
			verbZh = "已取消归档"
		}
		return map[string]interface{}{
			"status":  "success",
			"message": emailLocalized(lang, fmt.Sprintf("%s email: %s", verbEn, msg.Subject), fmt.Sprintf("%s邮件：%s", verbZh, msg.Subject)),
			"email":   emailMessageForResult(*msg, false),
		}, nil
	case "label":
		id := firstCompatString(args, "id", "email_id", "emailId", "message_id", "messageId")
		if id == "" {
			return nil, errors.New("id is required")
		}
		add := normalizeStringList(stringSliceArg(args, "add_labels", "addLabels", "labels"))
		if label := strings.TrimSpace(firstCompatString(args, "label")); label != "" {
			add = mergeLabels(add, []string{label}, nil)
		}
		remove := normalizeStringList(stringSliceArg(args, "remove_labels", "removeLabels"))
		msg, err := t.service.Label(ctx, ownerID, id, add, remove)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, errors.New("email not found")
		}
		return map[string]interface{}{
			"status":  "success",
			"message": emailLocalized(lang, fmt.Sprintf("Updated labels for %s", msg.Subject), fmt.Sprintf("已更新邮件标签：%s", msg.Subject)),
			"email":   emailMessageForResult(*msg, false),
		}, nil
	case "summarize":
		opts := emailQueryOptionsFromArgs(args)
		summary, err := t.service.Summarize(ctx, ownerID, opts)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":              "success",
			"message":             emailSummaryMessage(lang, summary),
			"summary":             emailSummaryText(lang, summary),
			"total":               summary.Total,
			"count":               summary.Total,
			"unread":              summary.Unread,
			"high_priority_count": summary.HighPriority,
			"actionable_count":    summary.ActionableCount,
			"label_counts":        summary.LabelCounts,
			"top_action_items":    append([]string(nil), summary.TopActionItems...),
			"emails":              emailMessagesForResult(summary.HighlightMessages, false),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported email action %q", action)
	}
}

func RegisterEmailTool(registry *Registry, service EmailService) *EmailTool {
	if registry == nil || service == nil {
		return nil
	}
	tool := NewEmailTool(service)
	registry.Register(tool)
	return tool
}

func GetEmailTool(registry *Registry) *EmailTool {
	if registry == nil {
		return nil
	}
	tool := registry.Get("email")
	if tool == nil {
		return nil
	}
	emailTool, _ := tool.(*EmailTool)
	return emailTool
}

func emailQueryOptionsFromArgs(args map[string]interface{}) EmailQueryOptions {
	opts := EmailQueryOptions{
		Query: strings.TrimSpace(firstCompatString(args, "query", "q", "search")),
		From:  strings.TrimSpace(firstCompatString(args, "from", "sender")),
		Label: strings.TrimSpace(firstCompatString(args, "label", "tag")),
		Limit: compatInt(args, "limit", "max_results", "maxResults"),
	}
	if priority := strings.TrimSpace(firstCompatString(args, "priority")); priority != "" {
		opts.Priority = normalizeEmailPriority(priority)
	}
	if raw, ok := compatArgValue(args, "unread"); ok {
		value := compatBoolValue(raw, false)
		opts.Unread = &value
	}
	if raw, ok := compatArgValue(args, "archived"); ok {
		value := compatBoolValue(raw, false)
		opts.Archived = &value
	}
	return opts
}

func normalizeEmailAction(raw string, args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "open", "read", "show", "detail":
		return "get"
	case "find":
		return "search"
	case "tag", "add_label", "add_labels", "update_labels":
		return "label"
	case "summary", "digest":
		return "summarize"
	}
	switch action {
	case "", "list", "search", "filter", "get", "archive", "label", "summarize":
	default:
		return action
	}
	if action != "" {
		return action
	}
	if strings.TrimSpace(firstCompatString(args, "id", "email_id", "emailId", "message_id", "messageId")) != "" {
		return "get"
	}
	if strings.TrimSpace(firstCompatString(args, "query", "q", "search", "from", "sender", "label", "priority")) != "" {
		return "search"
	}
	return "list"
}

func emailMessageForResult(msg EmailMessage, includeBody bool) map[string]interface{} {
	result := map[string]interface{}{
		"id":           msg.ID,
		"subject":      msg.Subject,
		"sender_name":  msg.SenderName,
		"sender_email": msg.SenderEmail,
		"snippet":      msg.Snippet,
		"labels":       normalizeStringList(msg.Labels),
		"priority":     normalizeEmailPriority(msg.Priority),
		"unread":       msg.Unread,
		"archived":     msg.Archived,
		"received_at":  msg.ReceivedAt.UTC().Format(time.RFC3339),
	}
	if len(msg.Recipients) > 0 {
		result["recipients"] = normalizeStringList(msg.Recipients)
	}
	if includeBody {
		result["body"] = msg.Body
	}
	return result
}

func emailMessagesForResult(messages []EmailMessage, includeBody bool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		result = append(result, emailMessageForResult(msg, includeBody))
	}
	return result
}

func emailSummaryMessage(lang string, summary *EmailSummary) string {
	if summary == nil {
		return emailLocalized(lang, "Inbox summary ready.", "收件箱摘要已生成。")
	}
	return emailLocalized(
		lang,
		fmt.Sprintf("Inbox summary: %d messages, %d unread, %d high-priority, %d actionable.", summary.Total, summary.Unread, summary.HighPriority, summary.ActionableCount),
		fmt.Sprintf("收件箱摘要：%d 封邮件，%d 封未读，%d 封高优先级，%d 封需处理。", summary.Total, summary.Unread, summary.HighPriority, summary.ActionableCount),
	)
}

func emailSummaryText(lang string, summary *EmailSummary) string {
	if summary == nil || len(summary.TopActionItems) == 0 {
		return emailLocalized(lang, "No urgent email follow-up is needed right now.", "当前没有需要立刻跟进的紧急邮件。")
	}
	prefix := emailLocalized(lang, "Focus first on: ", "优先处理：")
	return prefix + strings.Join(summary.TopActionItems, "; ")
}

func emailLocalized(lang, en, zh string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		return zh
	}
	return en
}

func stringSliceArg(args map[string]interface{}, keys ...string) []string {
	raw, ok := compatArgValue(args, keys...)
	if !ok {
		return nil
	}
	return normalizeStringList(raw)
}

func normalizeStringList(raw interface{}) []string {
	result := make([]string, 0, 4)
	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range result {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		result = append(result, value)
	}
	switch typed := raw.(type) {
	case []string:
		for _, item := range typed {
			appendValue(item)
		}
	case []interface{}:
		for _, item := range typed {
			appendValue(fmt.Sprintf("%v", item))
		}
	case string:
		if strings.Contains(typed, ",") || strings.Contains(typed, "\n") {
			for _, part := range strings.FieldsFunc(typed, func(r rune) bool { return r == ',' || r == '\n' }) {
				appendValue(part)
			}
		} else {
			appendValue(typed)
		}
	}
	return result
}

func mergeLabels(current, add, remove []string) []string {
	result := normalizeStringList(current)
	for _, item := range normalizeStringList(add) {
		found := false
		for _, existing := range result {
			if strings.EqualFold(existing, item) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, item)
		}
	}
	if len(remove) == 0 {
		return result
	}
	filtered := make([]string, 0, len(result))
	for _, existing := range result {
		keep := true
		for _, item := range normalizeStringList(remove) {
			if strings.EqualFold(existing, item) {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, existing)
		}
	}
	return filtered
}

func compatArgBool(args map[string]interface{}, keys ...string) bool {
	value, ok := compatArgValue(args, keys...)
	if !ok {
		return false
	}
	return compatBoolValue(value, false)
}

func compatBoolValue(value interface{}, defaultValue bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return defaultValue
}

func normalizeProductivityOwnerID(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "default"
	}
	return ownerID
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

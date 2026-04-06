package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/spf13/cobra"
)

var (
	auditFollow           bool
	auditLines            int
	auditQuery            string
	auditPollInterval     = 250 * time.Millisecond
	auditExit             = os.Exit
	auditIPCRoundTripFunc = auditIPCRoundTrip
	auditIPCWatchFunc     = auditIPCWatch
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect local session audit activity",
	Long: `Inspect session audit events from local JSONL sidecar logs.

This command stays local to the machine and does not open any audit listener by default.`,
}

var auditTailCmd = &cobra.Command{
	Use:   "tail [conversation-id]",
	Short: "Show recent audit entries and optionally follow new ones",
	Args:  cobra.MaximumNArgs(1),
	Run:   runAuditTail,
}

var auditRecentCmd = &cobra.Command{
	Use:   "recent [conversation-id]",
	Short: "Fetch recent audit entries from the running Blue service over IPC",
	Args:  cobra.MaximumNArgs(1),
	Run:   runAuditRecentCommand,
}

var auditWatchCmd = &cobra.Command{
	Use:   "watch [conversation-id]",
	Short: "Stream audit entries from the running Blue service over audit IPC",
	Args:  cobra.MaximumNArgs(1),
	Run:   runAuditWatchCommand,
}

type auditTailOptions struct {
	DataDir      string
	Conversation string
	Query        string
	Lines        int
	Follow       bool
	PollInterval time.Duration
}

type auditSeenWindow struct {
	limit int
	order []string
	set   map[string]struct{}
}

func init() {
	auditTailCmd.Flags().BoolVarP(&auditFollow, "follow", "f", false, "follow new audit entries in real-time")
	auditTailCmd.Flags().IntVarP(&auditLines, "lines", "n", 20, "number of recent entries to show before following")
	auditTailCmd.Flags().StringVarP(&auditQuery, "query", "q", "", "filter entries by case-insensitive text match")
	auditRecentCmd.Flags().IntVarP(&auditLines, "lines", "n", 20, "number of recent entries to fetch")
	auditRecentCmd.Flags().StringVarP(&auditQuery, "query", "q", "", "filter entries by case-insensitive text match")
	auditWatchCmd.Flags().IntVarP(&auditLines, "lines", "n", 20, "number of recent entries to preload before watching")
	auditWatchCmd.Flags().StringVarP(&auditQuery, "query", "q", "", "filter entries by case-insensitive text match")

	auditCmd.AddCommand(auditRecentCmd)
	auditCmd.AddCommand(auditTailCmd)
	auditCmd.AddCommand(auditWatchCmd)
	rootCmd.AddCommand(auditCmd)
}

func runAuditTail(cmd *cobra.Command, args []string) {
	var conversationID string
	if len(args) > 0 {
		conversationID = strings.TrimSpace(args[0])
	}

	ctx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := auditTailOptions{
		DataDir:      getDataDir(),
		Conversation: conversationID,
		Query:        auditQuery,
		Lines:        auditLines,
		Follow:       auditFollow,
		PollInterval: auditPollInterval,
	}

	if jsonOutput && !opts.Follow {
		entries := make([]sessionaudit.Entry, 0, opts.Lines)
		if err := tailAuditEntries(ctx, opts, func(entry sessionaudit.Entry) error {
			entries = append(entries, entry)
			return nil
		}); err != nil {
			printAuditError("Failed to read audit logs", err)
			return
		}
		printJSON(map[string]interface{}{
			"conversation_id": opts.Conversation,
			"query":           strings.TrimSpace(opts.Query),
			"count":           len(entries),
			"entries":         entries,
		})
		return
	}

	if err := tailAuditEntries(ctx, opts, func(entry sessionaudit.Entry) error {
		printAuditEntry(entry, opts.Conversation != "")
		return nil
	}); err != nil {
		printAuditError("Failed to read audit logs", err)
	}
}

func runAuditRecentCommand(cmd *cobra.Command, args []string) {
	params := make(map[string]string)
	if len(args) > 0 {
		params["conversation_id"] = strings.TrimSpace(args[0])
	}
	if auditLines > 0 {
		params["limit"] = strconv.Itoa(auditLines)
	}
	if query := strings.TrimSpace(auditQuery); query != "" {
		params["query"] = query
	}
	params = prepareIPCParams(params)

	resp, err := auditIPCRoundTripFunc(&sockipc.Request{Cmd: "audit.recent", Params: params})
	if err != nil {
		if isConnectionError(err) {
			printAuditIPCUnavailable("audit.recent", err)
			auditExit(1)
			return
		}
		printAuditError("Failed to query audit IPC", err)
		auditExit(1)
		return
	}
	if resp.Status != "ok" {
		if strings.Contains(strings.ToLower(strings.TrimSpace(resp.Error)), "unknown cmd: audit.recent") {
			printAuditIPCDisabled("audit.recent")
			auditExit(1)
			return
		}
		if jsonOutput {
			printJSON(map[string]string{"error": resp.Error})
		} else {
			fmt.Printf("Error: %s\n", resp.Error)
		}
		auditExit(1)
		return
	}

	entries, err := decodeAuditEntriesPayload(resp.Data)
	if err != nil {
		printAuditError("Failed to decode audit IPC response", err)
		auditExit(1)
		return
	}

	conversationID := strings.TrimSpace(params["conversation_id"])
	if jsonOutput {
		printJSON(map[string]interface{}{
			"conversation_id": conversationID,
			"query":           strings.TrimSpace(params["query"]),
			"count":           len(entries),
			"entries":         entries,
		})
		return
	}
	if len(entries) == 0 {
		fmt.Println("No audit entries found")
		return
	}
	for _, entry := range entries {
		printAuditEntry(entry, conversationID != "")
	}
}

func runAuditWatchCommand(cmd *cobra.Command, args []string) {
	params := make(map[string]string)
	if len(args) > 0 {
		params["conversation_id"] = strings.TrimSpace(args[0])
	}
	if auditLines > 0 {
		params["limit"] = strconv.Itoa(auditLines)
	}
	if query := strings.TrimSpace(auditQuery); query != "" {
		params["query"] = query
	}
	params = prepareIPCParams(params)

	ctx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	conversationID := strings.TrimSpace(params["conversation_id"])
	err := auditIPCWatchFunc(ctx, &sockipc.Request{Cmd: "audit.watch", Params: params}, func(resp *sockipc.Response) error {
		if resp == nil {
			return nil
		}
		if resp.Status != "ok" {
			if strings.Contains(strings.ToLower(strings.TrimSpace(resp.Error)), "unknown cmd: audit.watch") {
				printAuditIPCDisabled("audit.watch")
				auditExit(1)
				return nil
			}
			if jsonOutput {
				printJSON(map[string]string{"error": resp.Error})
			} else {
				fmt.Printf("Error: %s\n", resp.Error)
			}
			auditExit(1)
			return nil
		}

		entries, err := decodeAuditStreamEntries(resp.Data)
		if err != nil {
			printAuditError("Failed to decode audit watch response", err)
			auditExit(1)
			return nil
		}
		for _, entry := range entries {
			printAuditEntry(entry, conversationID != "")
		}
		return nil
	})
	if err == nil {
		return
	}
	if isConnectionError(err) {
		printAuditIPCUnavailable("audit.watch", err)
		auditExit(1)
		return
	}
	printAuditError("Failed to stream audit IPC", err)
	auditExit(1)
}

func tailAuditEntries(ctx context.Context, opts auditTailOptions, emit func(sessionaudit.Entry) error) error {
	if emit == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeAuditTailOptions(opts)

	store, err := sessionaudit.NewJSONLStore(opts.DataDir, sessionaudit.DefaultStoreConfig())
	if err != nil {
		return err
	}
	defer store.Close()

	seen := newAuditSeenWindow(followWindowSize(opts))

	initial, err := loadRecentAuditEntries(ctx, store, opts, initialReadSize(opts))
	if err != nil {
		return err
	}
	initial = filterAuditEntries(initial, opts.Query)
	for _, entry := range initial {
		seen.Mark(entry)
	}
	if len(initial) > opts.Lines {
		initial = initial[len(initial)-opts.Lines:]
	}
	for _, entry := range initial {
		if err := emit(entry); err != nil {
			return err
		}
	}
	if !opts.Follow {
		return nil
	}

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			entries, err := loadRecentAuditEntries(ctx, store, opts, followWindowSize(opts))
			if err != nil {
				return err
			}
			entries = filterAuditEntries(entries, opts.Query)
			for _, entry := range entries {
				if !seen.Mark(entry) {
					continue
				}
				if err := emit(entry); err != nil {
					return err
				}
			}
		}
	}
}

func normalizeAuditTailOptions(opts auditTailOptions) auditTailOptions {
	opts.DataDir = strings.TrimSpace(opts.DataDir)
	opts.Conversation = strings.TrimSpace(opts.Conversation)
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Lines <= 0 {
		opts.Lines = 20
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 250 * time.Millisecond
	}
	return opts
}

func loadRecentAuditEntries(ctx context.Context, store *sessionaudit.Store, opts auditTailOptions, limit int) ([]sessionaudit.Entry, error) {
	if limit <= 0 {
		limit = opts.Lines
	}
	entries, err := store.Recent(ctx, opts.Conversation, limit)
	if err != nil {
		return nil, err
	}
	slices.Reverse(entries)
	sortAuditEntries(entries)
	return entries, nil
}

func sortAuditEntries(entries []sessionaudit.Entry) {
	slices.SortFunc(entries, func(a, b sessionaudit.Entry) int {
		switch {
		case a.CreatedAt.Before(b.CreatedAt):
			return -1
		case a.CreatedAt.After(b.CreatedAt):
			return 1
		}
		if cmp := strings.Compare(a.ConversationID, b.ConversationID); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.ID, b.ID)
	})
}

func initialReadSize(opts auditTailOptions) int {
	if opts.Query != "" || opts.Follow {
		return maxInt(opts.Lines*10, 200)
	}
	return opts.Lines
}

func followWindowSize(opts auditTailOptions) int {
	return maxInt(opts.Lines*10, 200)
}

func filterAuditEntries(entries []sessionaudit.Entry, query string) []sessionaudit.Entry {
	query = strings.TrimSpace(query)
	if query == "" {
		return entries
	}
	filtered := make([]sessionaudit.Entry, 0, len(entries))
	for _, entry := range entries {
		if auditEntryMatchesQuery(entry, query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func auditEntryMatchesQuery(entry sessionaudit.Entry, query string) bool {
	text := strings.ToLower(strings.Join([]string{
		entry.ConversationID,
		entry.SessionID,
		entry.UserID,
		entry.Source,
		entry.EventType,
		entry.Role,
		entry.ToolName,
		entry.Payload,
	}, " "))
	for _, term := range strings.Fields(strings.ToLower(strings.TrimSpace(query))) {
		if term == "" {
			continue
		}
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func newAuditSeenWindow(limit int) *auditSeenWindow {
	if limit <= 0 {
		limit = 512
	}
	return &auditSeenWindow{
		limit: limit,
		order: make([]string, 0, limit),
		set:   make(map[string]struct{}, limit),
	}
}

func (w *auditSeenWindow) Mark(entry sessionaudit.Entry) bool {
	if w == nil {
		return true
	}
	key := auditEntryKey(entry)
	if _, exists := w.set[key]; exists {
		return false
	}
	w.set[key] = struct{}{}
	w.order = append(w.order, key)
	if len(w.order) > w.limit {
		evicted := w.order[0]
		w.order = w.order[1:]
		delete(w.set, evicted)
	}
	return true
}

func auditEntryKey(entry sessionaudit.Entry) string {
	if id := strings.TrimSpace(entry.ID); id != "" {
		return id
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s",
		entry.ConversationID,
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
		entry.EventType,
		entry.ToolName,
		entry.Payload,
	)
}

func printAuditEntry(entry sessionaudit.Entry, singleConversation bool) {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(entry)
		return
	}

	timestamp := entry.CreatedAt.UTC().Format(time.RFC3339)
	roleEvent := entry.Role
	if strings.TrimSpace(entry.EventType) != "" {
		roleEvent = roleEvent + "/" + entry.EventType
	}
	if strings.TrimSpace(roleEvent) == "/" || strings.TrimSpace(roleEvent) == "" {
		roleEvent = strings.TrimSpace(entry.EventType)
	}

	var parts []string
	parts = append(parts, timestamp)
	if !singleConversation && strings.TrimSpace(entry.ConversationID) != "" {
		parts = append(parts, shortAuditConversationID(entry.ConversationID))
	}
	if strings.TrimSpace(roleEvent) != "" {
		parts = append(parts, roleEvent)
	}
	if strings.TrimSpace(entry.ToolName) != "" {
		parts = append(parts, "tool="+entry.ToolName)
	}

	line := strings.Join(parts, " ")
	payload := clipAuditPayload(entry.Payload, 220)
	if payload != "" {
		line = line + " " + payload
	}
	fmt.Println(line)
}

func shortAuditConversationID(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if len(conversationID) <= 12 {
		return conversationID
	}
	return conversationID[:12]
}

func clipAuditPayload(payload string, maxLen int) string {
	payload = strings.Join(strings.Fields(strings.TrimSpace(payload)), " ")
	if payload == "" || maxLen <= 0 {
		return payload
	}
	runes := []rune(payload)
	if len(runes) <= maxLen {
		return payload
	}
	return string(runes[:maxLen]) + "..."
}

func printAuditError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]string{
			"error":  msg,
			"detail": err.Error(),
		})
		return
	}
	fmt.Printf("%s: %v\n", msg, err)
}

func printAuditIPCUnavailable(cmd string, err error) {
	if jsonOutput {
		printJSON(map[string]string{
			"error": fmt.Sprintf("running Blue service is required for %s: %v", cmd, err),
		})
		return
	}
	fmt.Printf("Error: running Blue service is required for %s: %v\n", cmd, err)
}

func printAuditIPCDisabled(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		cmd = "audit.recent"
	}
	msg := fmt.Sprintf("audit IPC is not enabled in the running Blue service; set session.audit.ipc_enabled: true to enable %s", cmd)
	if jsonOutput {
		printJSON(map[string]string{"error": msg})
		return
	}
	fmt.Printf("Error: %s\n", msg)
}

func decodeAuditEntriesPayload(data map[string]string) ([]sessionaudit.Entry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	raw := strings.TrimSpace(data["entries"])
	if raw == "" {
		return nil, nil
	}
	var entries []sessionaudit.Entry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func decodeAuditEntryPayload(data map[string]string) (*sessionaudit.Entry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	raw := strings.TrimSpace(data["entry"])
	if raw == "" {
		return nil, nil
	}
	var entry sessionaudit.Entry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func decodeAuditStreamEntries(data map[string]string) ([]sessionaudit.Entry, error) {
	entries, err := decodeAuditEntriesPayload(data)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		return entries, nil
	}
	entry, err := decodeAuditEntryPayload(data)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	return []sessionaudit.Entry{*entry}, nil
}

func auditIPCRoundTrip(req *sockipc.Request) (*sockipc.Response, error) {
	return ipcRoundTripWithSocketName(req, candidateAuditIPCSocketPaths(), "session_audit.sock")
}

func auditIPCWatch(ctx context.Context, req *sockipc.Request, onResp func(*sockipc.Response) error) error {
	conn, err := dialSockWithCandidates(candidateAuditIPCSocketPaths(), "session_audit.sock")
	if err != nil {
		return err
	}
	defer conn.Close()

	if ctx == nil {
		ctx = context.Background()
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	if timeout := ipcRequestTimeout(req); timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	if err := sockipc.WriteJSON(conn, req); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	_ = conn.SetWriteDeadline(time.Time{})

	for {
		resp, err := sockipc.ReadJSON[sockipc.Response](conn)
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}
		if onResp != nil {
			if err := onResp(resp); err != nil {
				return err
			}
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

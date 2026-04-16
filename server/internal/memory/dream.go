package memory

import (
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	sessionctx "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

var ErrDreamDisabled = errors.New("dream service disabled")

type DreamConfig struct {
	Enabled               bool   `json:"enabled" yaml:"enabled"`
	ArchiveDir            string `json:"archive_dir" yaml:"archive_dir"`
	Schedule              string `json:"schedule" yaml:"schedule"`
	PromoteDailyAfterDays int    `json:"promote_daily_after_days" yaml:"promote_daily_after_days"`
	ArchiveDailyAfterDays int    `json:"archive_daily_after_days" yaml:"archive_daily_after_days"`
	SessionMinMessages    int    `json:"session_min_messages" yaml:"session_min_messages"`
	MaxPromotionsPerRun   int    `json:"max_promotions_per_run" yaml:"max_promotions_per_run"`
}

func DefaultDreamConfig() DreamConfig {
	return DreamConfig{
		Enabled:               true,
		Schedule:              "0 30 3 * * *",
		PromoteDailyAfterDays: 7,
		ArchiveDailyAfterDays: 30,
		SessionMinMessages:    4,
		MaxPromotionsPerRun:   5,
	}
}

type DreamCandidate struct {
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

type DreamSessionCapsule struct {
	ArtifactID   string           `json:"artifact_id"`
	WorkspaceKey string           `json:"workspace_key"`
	SessionID    string           `json:"session_id"`
	Reason       string           `json:"reason"`
	Title        string           `json:"title,omitempty"`
	Summary      string           `json:"summary,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	MessageCount int              `json:"message_count"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	EndedAt      time.Time        `json:"ended_at"`
	Excerpt      string           `json:"excerpt,omitempty"`
	Candidates   []DreamCandidate `json:"candidates,omitempty"`
}

type DreamRunResult struct {
	RunID              string `json:"run_id"`
	PromotedCount      int    `json:"promoted_count"`
	ArchivedDailyCount int    `json:"archived_daily_count"`
	ProcessedCapsules  int    `json:"processed_capsules"`
}

type DreamStatus struct {
	Enabled           bool       `json:"enabled"`
	ArchiveDir        string     `json:"archive_dir"`
	LastRunAt         *time.Time `json:"last_run_at,omitempty"`
	PendingCapsules   int        `json:"pending_capsules"`
	ArchivedDailyLogs int        `json:"archived_daily_logs"`
	PromotedCount     int        `json:"promoted_count"`
}

type dreamStatusFile struct {
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
}

type DreamService struct {
	config       DreamConfig
	layered      *LayeredMemoryService
	workspaceDir string
	workspaceKey string
	archiveRoot  string
	scorer       *ImportanceScorer

	mu sync.Mutex
}

func NewDreamService(layered *LayeredMemoryService, workspaceDir string, cfg DreamConfig) (*DreamService, error) {
	defaults := DefaultDreamConfig()
	if cfg.Schedule == "" {
		cfg.Schedule = defaults.Schedule
	}
	if cfg.PromoteDailyAfterDays <= 0 {
		cfg.PromoteDailyAfterDays = defaults.PromoteDailyAfterDays
	}
	if cfg.ArchiveDailyAfterDays <= 0 {
		cfg.ArchiveDailyAfterDays = defaults.ArchiveDailyAfterDays
	}
	if cfg.SessionMinMessages <= 0 {
		cfg.SessionMinMessages = defaults.SessionMinMessages
	}
	if cfg.MaxPromotionsPerRun <= 0 {
		cfg.MaxPromotionsPerRun = defaults.MaxPromotionsPerRun
	}
	if strings.TrimSpace(cfg.ArchiveDir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		cfg.ArchiveDir = filepath.Join(home, ".zimaos-blue", "archives", "dream")
	}

	workspaceKey := dreamWorkspaceKey(workspaceDir)
	archiveRoot := filepath.Join(cfg.ArchiveDir, workspaceKey)
	for _, dir := range []string{
		filepath.Join(archiveRoot, "sessions"),
		filepath.Join(archiveRoot, "daily"),
		filepath.Join(archiveRoot, "manifest"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir dream archive dir %q: %w", dir, err)
		}
	}

	return &DreamService{
		config:       cfg,
		layered:      layered,
		workspaceDir: workspaceDir,
		workspaceKey: workspaceKey,
		archiveRoot:  archiveRoot,
		scorer:       NewImportanceScorer(DefaultImportanceConfig()),
	}, nil
}

func (s *DreamService) WorkspaceKey() string {
	if s == nil {
		return ""
	}
	return s.workspaceKey
}

func (s *DreamService) WorkspaceArchiveDir() string {
	if s == nil {
		return ""
	}
	return s.archiveRoot
}

func (s *DreamService) Config() DreamConfig {
	if s == nil {
		return DreamConfig{}
	}
	return s.config
}

func (s *DreamService) ArchiveSession(ctx context.Context, sess *session.Session, reason session.EndReason) (string, error) {
	if s == nil {
		return "", nil
	}
	if !s.config.Enabled {
		return "", nil
	}
	if reason == session.EndReasonDelete || sess == nil {
		return "", nil
	}

	messages := filterDreamMessages(sess.GetMessages())
	if len(messages) < s.config.SessionMinMessages {
		return "", nil
	}

	capsule := s.buildSessionCapsule(sess, messages, reason)
	path := s.sessionCapsulePath(capsule.EndedAt, reason, capsule.SessionID, capsule.Summary, capsule.Excerpt)
	artifactID, err := filepath.Rel(s.archiveRoot, path)
	if err != nil {
		return "", fmt.Errorf("relative artifact path: %w", err)
	}
	capsule.ArtifactID = filepath.ToSlash(artifactID)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir session capsule dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return capsule.ArtifactID, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat session capsule: %w", err)
	}

	if err := writeGzipJSON(path, capsule); err != nil {
		return "", err
	}
	return capsule.ArtifactID, nil
}

func (s *DreamService) Status(ctx context.Context) (DreamStatus, error) {
	_ = ctx
	if s == nil {
		return DreamStatus{}, nil
	}

	processed, err := s.loadManifestSet(s.processedManifestPath())
	if err != nil {
		return DreamStatus{}, err
	}
	promoted, err := s.loadManifestSet(s.promotedManifestPath())
	if err != nil {
		return DreamStatus{}, err
	}
	lastRunAt, err := s.loadLastRun()
	if err != nil {
		return DreamStatus{}, err
	}

	pending := 0
	files, err := filepath.Glob(filepath.Join(s.sessionArchiveRoot(), "*", "*", "*", "*.json.gz"))
	if err != nil {
		return DreamStatus{}, fmt.Errorf("glob session capsules: %w", err)
	}
	for _, path := range files {
		artifactID, relErr := filepath.Rel(s.archiveRoot, path)
		if relErr != nil {
			continue
		}
		if _, ok := processed[filepath.ToSlash(artifactID)]; ok {
			continue
		}
		pending++
	}

	archivedDaily, err := filepath.Glob(filepath.Join(s.dailyArchiveRoot(), "*", "*", "*.md.gz"))
	if err != nil {
		return DreamStatus{}, fmt.Errorf("glob daily archives: %w", err)
	}

	return DreamStatus{
		Enabled:           s.config.Enabled,
		ArchiveDir:        s.config.ArchiveDir,
		LastRunAt:         lastRunAt,
		PendingCapsules:   pending,
		ArchivedDailyLogs: len(archivedDaily),
		PromotedCount:     len(promoted),
	}, nil
}

func (s *DreamService) RunConsolidation(ctx context.Context) (DreamRunResult, error) {
	if s == nil {
		return DreamRunResult{}, nil
	}
	if !s.config.Enabled {
		return DreamRunResult{}, ErrDreamDisabled
	}
	if s.layered == nil {
		return DreamRunResult{}, errors.New("dream layered memory service not configured")
	}

	runTime := timeutil.NowTime().UTC()
	result := DreamRunResult{RunID: runTime.Format("20060102T150405Z")}

	s.mu.Lock()
	defer s.mu.Unlock()

	processed, err := s.loadManifestSet(s.processedManifestPath())
	if err != nil {
		return result, err
	}
	promoted, err := s.loadManifestSet(s.promotedManifestPath())
	if err != nil {
		return result, err
	}

	candidates, processedCapsules, processedDaily, err := s.collectCandidates(ctx, processed)
	if err != nil {
		return result, err
	}

	selected := s.selectCandidates(candidates, promoted)
	if len(selected) > s.config.MaxPromotionsPerRun {
		selected = selected[:s.config.MaxPromotionsPerRun]
	}

	for _, candidate := range selected {
		fingerprint := dreamFingerprint(candidate.Content)
		if _, ok := promoted[fingerprint]; ok {
			continue
		}
		if err := s.layered.PromoteToLongTerm(ctx, candidate.Content, candidate.Category); err != nil {
			return result, fmt.Errorf("promote dream memory: %w", err)
		}
		if err := s.appendManifestLine(s.promotedManifestPath(), fingerprint); err != nil {
			return result, err
		}
		promoted[fingerprint] = struct{}{}
		result.PromotedCount++
	}

	for _, artifactID := range processedCapsules {
		if _, ok := processed[artifactID]; ok {
			continue
		}
		if err := s.appendManifestLine(s.processedManifestPath(), artifactID); err != nil {
			return result, err
		}
		processed[artifactID] = struct{}{}
		result.ProcessedCapsules++
	}
	for _, artifactID := range processedDaily {
		if _, ok := processed[artifactID]; ok {
			continue
		}
		if err := s.appendManifestLine(s.processedManifestPath(), artifactID); err != nil {
			return result, err
		}
		processed[artifactID] = struct{}{}
	}

	archivedDailyCount, err := s.archiveEligibleDailyLogs(ctx)
	if err != nil {
		return result, err
	}
	result.ArchivedDailyCount = archivedDailyCount

	if err := s.saveLastRun(runTime); err != nil {
		return result, err
	}
	return result, nil
}

func (s *DreamService) buildSessionCapsule(sess *session.Session, messages []sessionctx.Message, reason session.EndReason) DreamSessionCapsule {
	endedAt := timeutil.NowTime().UTC()
	summary := strings.TrimSpace(sess.GetSummary())
	excerpt := buildDreamExcerpt(messages, 6)
	candidates := s.extractSessionCandidates(summary, messages)

	return DreamSessionCapsule{
		WorkspaceKey: s.workspaceKey,
		SessionID:    sess.ID.String(),
		Reason:       string(reason),
		Title:        strings.TrimSpace(sess.GetTitle()),
		Summary:      summary,
		Tags:         sess.GetTags(),
		MessageCount: len(messages),
		CreatedAt:    sess.CreatedAt.UTC(),
		UpdatedAt:    sess.UpdatedAt.UTC(),
		EndedAt:      endedAt,
		Excerpt:      excerpt,
		Candidates:   candidates,
	}
}

func (s *DreamService) extractSessionCandidates(summary string, messages []sessionctx.Message) []DreamCandidate {
	if summary = strings.TrimSpace(summary); summary != "" {
		return []DreamCandidate{{
			Content:   summary,
			Category:  "Session Summary",
			Source:    "session-summary",
			CreatedAt: timeutil.NowTime().UTC(),
		}}
	}

	candidates := make([]DreamCandidate, 0, len(messages))
	seen := make(map[string]struct{})
	for _, msg := range messages {
		content := normalizeDreamText(msg.Content)
		if content == "" || !dreamLooksImportant(content) {
			continue
		}
		fingerprint := dreamFingerprint(content)
		if _, ok := seen[fingerprint]; ok {
			continue
		}
		seen[fingerprint] = struct{}{}
		candidates = append(candidates, DreamCandidate{
			Content:   content,
			Category:  "Conversation",
			Source:    string(msg.Role),
			CreatedAt: timeutil.NowTime().UTC(),
		})
	}
	if len(candidates) == 0 {
		excerpt := buildDreamExcerpt(messages, 4)
		if excerpt != "" {
			candidates = append(candidates, DreamCandidate{
				Content:   excerpt,
				Category:  "Conversation",
				Source:    "excerpt",
				CreatedAt: timeutil.NowTime().UTC(),
			})
		}
	}
	return candidates
}

func (s *DreamService) collectCandidates(ctx context.Context, processed map[string]struct{}) ([]DreamCandidate, []string, []string, error) {
	candidates := make([]DreamCandidate, 0)
	processedCapsules := make([]string, 0)
	processedDaily := make([]string, 0)

	sessionFiles, err := filepath.Glob(filepath.Join(s.sessionArchiveRoot(), "*", "*", "*", "*.json.gz"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("glob session capsules: %w", err)
	}
	sort.Strings(sessionFiles)
	for _, path := range sessionFiles {
		artifactID, relErr := filepath.Rel(s.archiveRoot, path)
		if relErr != nil {
			continue
		}
		artifactID = filepath.ToSlash(artifactID)
		if _, ok := processed[artifactID]; ok {
			continue
		}
		var capsule DreamSessionCapsule
		if err := readGzipJSON(path, &capsule); err != nil {
			return nil, nil, nil, err
		}
		for _, candidate := range capsule.Candidates {
			candidate.Content = normalizeDreamText(candidate.Content)
			if candidate.Content == "" {
				continue
			}
			if candidate.CreatedAt.IsZero() {
				candidate.CreatedAt = capsule.EndedAt
			}
			if candidate.Category == "" {
				candidate.Category = "Session Summary"
			}
			candidates = append(candidates, candidate)
		}
		processedCapsules = append(processedCapsules, artifactID)
	}

	dates, err := s.layered.ListDailyLogs(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list daily logs: %w", err)
	}
	cutoff := timeutil.NowTime().UTC().AddDate(0, 0, -s.config.PromoteDailyAfterDays)
	extractor := NewMemoryExtractor(s.layered)
	for _, dateStr := range dates {
		if !dreamDateEligible(dateStr, cutoff) {
			continue
		}
		artifactID := "daily:" + dateStr
		if _, ok := processed[artifactID]; ok {
			continue
		}
		content, err := s.layered.GetDailyLog(ctx, dateStr)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, entry := range extractor.parseLogEntries(content) {
			text := normalizeDreamText(entry.content)
			if text == "" {
				continue
			}
			candidates = append(candidates, DreamCandidate{
				Content:   text,
				Category:  extractor.inferCategory(text, entry.tags),
				Source:    "daily:" + dateStr,
				CreatedAt: dreamLogEntryTime(dateStr, entry.timestamp),
			})
		}
		processedDaily = append(processedDaily, artifactID)
	}
	return candidates, processedCapsules, processedDaily, nil
}

func (s *DreamService) selectCandidates(candidates []DreamCandidate, promoted map[string]struct{}) []DreamCandidate {
	type scoredCandidate struct {
		DreamCandidate
		score float32
	}

	scored := make([]scoredCandidate, 0, len(candidates))
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		fingerprint := dreamFingerprint(candidate.Content)
		if fingerprint == "" {
			continue
		}
		if _, ok := promoted[fingerprint]; ok {
			continue
		}
		if _, ok := seen[fingerprint]; ok {
			continue
		}
		seen[fingerprint] = struct{}{}

		scored = append(scored, scoredCandidate{
			DreamCandidate: candidate,
			score: s.scorer.Score(&MemoryChunk{
				Content:   candidate.Content,
				CreatedAt: candidate.CreatedAt,
			}),
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].CreatedAt.After(scored[j].CreatedAt)
		}
		return scored[i].score > scored[j].score
	})

	out := make([]DreamCandidate, 0, len(scored))
	for _, candidate := range scored {
		out = append(out, candidate.DreamCandidate)
	}
	return out
}

func (s *DreamService) archiveEligibleDailyLogs(ctx context.Context) (int, error) {
	dates, err := s.layered.ListDailyLogs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list daily logs for archive: %w", err)
	}
	cutoff := timeutil.NowTime().UTC().AddDate(0, 0, -s.config.ArchiveDailyAfterDays)
	archived := 0
	for _, dateStr := range dates {
		if !dreamDateEligible(dateStr, cutoff) {
			continue
		}
		created, err := s.archiveDailyLog(ctx, dateStr)
		if err != nil {
			return archived, err
		}
		if created {
			archived++
		}
	}
	return archived, nil
}

func (s *DreamService) archiveDailyLog(ctx context.Context, dateStr string) (bool, error) {
	content, err := s.layered.GetDailyLog(ctx, dateStr)
	if err != nil {
		return false, err
	}

	logDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false, fmt.Errorf("parse daily log date %q: %w", dateStr, err)
	}
	dest := filepath.Join(s.dailyArchiveRoot(), logDate.UTC().Format("2006"), logDate.UTC().Format("01"), dateStr+".md.gz")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, fmt.Errorf("mkdir daily archive dir: %w", err)
	}
	created := false
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		if err := writeGzipBytes(dest, []byte(content)); err != nil {
			return false, err
		}
		created = true
	} else if err != nil {
		return false, fmt.Errorf("stat daily archive: %w", err)
	}

	src := filepath.Join(s.layered.dailyLogPath, dateStr+".md")
	if err := os.Remove(src); err != nil && !errors.Is(err, os.ErrNotExist) {
		return created, fmt.Errorf("remove daily log %q: %w", src, err)
	}
	return created, nil
}

func (s *DreamService) sessionCapsulePath(endedAt time.Time, reason session.EndReason, parts ...string) string {
	hash := dreamFingerprint(strings.Join(parts, "|"))
	date := endedAt.UTC()
	filename := fmt.Sprintf("%s_%s_%s.json.gz", date.Format("20060102T150405Z"), reason, hash)
	return filepath.Join(s.sessionArchiveRoot(), date.Format("2006"), date.Format("01"), date.Format("02"), filename)
}

func (s *DreamService) sessionArchiveRoot() string {
	return filepath.Join(s.archiveRoot, "sessions")
}

func (s *DreamService) dailyArchiveRoot() string {
	return filepath.Join(s.archiveRoot, "daily")
}

func (s *DreamService) manifestDir() string {
	return filepath.Join(s.archiveRoot, "manifest")
}

func (s *DreamService) processedManifestPath() string {
	return filepath.Join(s.manifestDir(), "processed_artifacts.log")
}

func (s *DreamService) promotedManifestPath() string {
	return filepath.Join(s.manifestDir(), "promoted_fingerprints.log")
}

func (s *DreamService) statusPath() string {
	return filepath.Join(s.archiveRoot, "status.json")
}

func (s *DreamService) appendManifestLine(path, line string) error {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open manifest %q: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("append manifest %q: %w", path, err)
	}
	return nil
}

func (s *DreamService) loadManifestSet(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]struct{}), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	out := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out[line] = struct{}{}
	}
	return out, nil
}

func (s *DreamService) saveLastRun(at time.Time) error {
	payload, err := json.MarshalIndent(dreamStatusFile{LastRunAt: &at}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dream status: %w", err)
	}
	if err := os.WriteFile(s.statusPath(), payload, 0o644); err != nil {
		return fmt.Errorf("write dream status: %w", err)
	}
	return nil
}

func (s *DreamService) loadLastRun() (*time.Time, error) {
	data, err := os.ReadFile(s.statusPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read dream status: %w", err)
	}
	var status dreamStatusFile
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("decode dream status: %w", err)
	}
	return status.LastRunAt, nil
}

type DreamSessionHook struct {
	service *DreamService
}

func NewDreamSessionHook(service *DreamService) *DreamSessionHook {
	return &DreamSessionHook{service: service}
}

func (h *DreamSessionHook) OnSessionEnd(ctx context.Context, sess *session.Session, reason session.EndReason) error {
	if h == nil || h.service == nil {
		return nil
	}
	_, err := h.service.ArchiveSession(ctx, sess, reason)
	return err
}

func filterDreamMessages(messages []sessionctx.Message) []sessionctx.Message {
	filtered := make([]sessionctx.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == sessionctx.RoleSystem {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}

func buildDreamExcerpt(messages []sessionctx.Message, maxMessages int) string {
	if maxMessages <= 0 {
		maxMessages = 4
	}
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}
	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		content := normalizeDreamText(msg.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", msg.Role, content))
	}
	return strings.Join(lines, "\n")
}

func normalizeDreamText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return strings.Join(strings.Fields(content), " ")
}

func dreamLooksImportant(content string) bool {
	lower := strings.ToLower(content)
	for _, keyword := range DefaultImportanceConfig().ImportantKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func dreamLogEntryTime(dateStr string, at time.Time) time.Time {
	logDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return timeutil.NowTime().UTC()
	}
	return time.Date(logDate.Year(), logDate.Month(), logDate.Day(), at.Hour(), at.Minute(), at.Second(), 0, time.UTC)
}

func dreamDateEligible(dateStr string, cutoff time.Time) bool {
	logDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	return !logDate.After(cutoff)
}

func dreamWorkspaceKey(workspaceDir string) string {
	cleaned := filepath.Clean(strings.TrimSpace(workspaceDir))
	base := sanitizeDreamToken(filepath.Base(cleaned))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "workspace"
	}
	return base + "-" + dreamFingerprint(cleaned)[:10]
}

func sanitizeDreamToken(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func dreamFingerprint(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeGzipJSON(path string, payload interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create gzip json %q: %w", path, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	if err := json.NewEncoder(gw).Encode(payload); err != nil {
		gw.Close()
		return fmt.Errorf("encode gzip json %q: %w", path, err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("close gzip writer %q: %w", path, err)
	}
	return nil
}

func writeGzipBytes(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create gzip file %q: %w", path, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	if _, err := gw.Write(data); err != nil {
		gw.Close()
		return fmt.Errorf("write gzip file %q: %w", path, err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("close gzip file %q: %w", path, err)
	}
	return nil
}

func readGzipJSON(path string, dest interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open gzip json %q: %w", path, err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip reader %q: %w", path, err)
	}
	defer gr.Close()

	if err := json.NewDecoder(gr).Decode(dest); err != nil {
		return fmt.Errorf("decode gzip json %q: %w", path, err)
	}
	return nil
}

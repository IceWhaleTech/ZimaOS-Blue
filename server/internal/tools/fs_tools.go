package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxFSToolBytes         = 2 << 20 // 2 MiB
	maxFSToolEntries       = 5000
	maxFSSearchResults     = 200
	defaultFSToolFindDepth = 6
	defaultFSToolLsDepth   = 1
	defaultFSToolLsEntries = 200
)

var errFSToolWalkDone = errors.New("fs_tool_walk_done")

type fsToolScope struct {
	roots     []string
	approvals *ApprovalManager
	dirStore  *DirAllowlistStore
}

func newFSToolScope(allowedPaths []string) *fsToolScope {
	roots := make([]string, 0, len(allowedPaths))
	for _, raw := range allowedPaths {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		roots = append(roots, filepath.Clean(abs))
	}
	if len(roots) == 0 {
		cwd, err := os.Getwd()
		if err == nil {
			if abs, absErr := filepath.Abs(cwd); absErr == nil {
				roots = append(roots, filepath.Clean(abs))
			}
		}
	}
	if len(roots) == 0 {
		roots = append(roots, ".")
	}
	return &fsToolScope{roots: roots}
}

func (s *fsToolScope) withApprovalFlow(approvals *ApprovalManager, dirStore *DirAllowlistStore) *fsToolScope {
	if s == nil {
		return &fsToolScope{approvals: approvals, dirStore: dirStore}
	}
	return &fsToolScope{
		roots:     append([]string(nil), s.roots...),
		approvals: approvals,
		dirStore:  dirStore,
	}
}

func (s *fsToolScope) rootsWithContext(ctx context.Context) []string {
	if s == nil {
		return nil
	}
	roots := make([]string, 0, len(s.roots))
	seen := make(map[string]struct{}, len(s.roots))
	for _, r := range s.roots {
		clean := filepath.Clean(r)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}

	extraRoots, _ := GetFSScope(ctx)
	for _, r := range extraRoots {
		clean := filepath.Clean(strings.TrimSpace(r))
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}
	return roots
}

func (s *fsToolScope) aliasesWithContext(ctx context.Context) map[string]string {
	_, aliases := GetFSScope(ctx)
	if len(aliases) == 0 {
		return nil
	}
	out := make(map[string]string, len(aliases))
	for rawAlias, rawPath := range aliases {
		alias := normalizeFSAliasKey(rawAlias)
		if alias == "" {
			continue
		}
		path := filepath.Clean(strings.TrimSpace(rawPath))
		if path == "" {
			continue
		}
		out[alias] = path
	}
	return out
}

func (s *fsToolScope) resolvePathWithContext(ctx context.Context, toolName, raw string, allowDot bool) (absPath string, relPath string, root string, err error) {
	if s == nil {
		return "", "", "", errors.New("workspace root is not configured")
	}
	roots := s.rootsWithContext(ctx)
	scope := &fsToolScope{
		roots:     roots,
		approvals: s.approvals,
		dirStore:  s.dirStore,
	}
	candidate := strings.TrimSpace(raw)
	if aliases := s.aliasesWithContext(ctx); len(aliases) > 0 {
		if resolved, ok := resolveFSAliasPath(candidate, aliases); ok {
			candidate = resolved
		}
	}
	return scope.resolvePathWithApproval(ctx, toolName, candidate, allowDot)
}

func resolveFSAliasPath(raw string, aliases map[string]string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(aliases) == 0 {
		return "", false
	}

	// @alias/path
	if strings.HasPrefix(trimmed, "@") {
		aliasRef := strings.TrimPrefix(trimmed, "@")
		alias, rest := splitFSAliasRef(aliasRef)
		root, ok := aliases[normalizeFSAliasKey(alias)]
		if !ok {
			return "", false
		}
		return joinFSAliasRoot(root, rest), true
	}

	// alias:path (explicit alias syntax)
	if idx := strings.IndexByte(trimmed, ':'); idx > 0 {
		alias := normalizeFSAliasKey(trimmed[:idx])
		if alias == "" {
			return "", false
		}
		root, ok := aliases[alias]
		if !ok {
			return "", false
		}
		rest := trimmed[idx+1:]
		return joinFSAliasRoot(root, rest), true
	}

	return "", false
}

func splitFSAliasRef(ref string) (alias string, rest string) {
	for i, r := range ref {
		if r == '/' || r == '\\' {
			return ref[:i], ref[i+1:]
		}
	}
	return ref, ""
}

func joinFSAliasRoot(root string, rest string) string {
	cleanRoot := filepath.Clean(root)
	trimmedRest := strings.TrimLeft(rest, "/\\")
	if trimmedRest == "" {
		return cleanRoot
	}
	normalizedRest := strings.ReplaceAll(trimmedRest, "\\", "/")
	joined := filepath.Join(cleanRoot, filepath.FromSlash(normalizedRest))
	return filepath.Clean(joined)
}

func (s *fsToolScope) resolvePath(raw string, allowDot bool) (absPath string, relPath string, root string, err error) {
	if s == nil || len(s.roots) == 0 {
		return "", "", "", errors.New("workspace root is not configured")
	}

	trimmed := strings.TrimSpace(raw)
	if strings.ContainsRune(trimmed, 0) {
		return "", "", "", errors.New("invalid path")
	}
	if trimmed == "" {
		if allowDot {
			return s.roots[0], ".", s.roots[0], nil
		}
		return "", "", "", errors.New("path is required")
	}

	if filepath.IsAbs(trimmed) {
		candidate := filepath.Clean(trimmed)
		for _, r := range s.roots {
			if fsPathWithinRoot(r, candidate) {
				rel, relErr := filepath.Rel(r, candidate)
				if relErr != nil {
					return "", "", "", relErr
				}
				if rel == "." && !allowDot {
					return "", "", "", errors.New("path must point to a file")
				}
				return candidate, filepath.ToSlash(rel), r, nil
			}
		}
		return "", "", "", errors.New("path escapes workspace root")
	}

	rel, relErr := cleanFSToolRelPath(trimmed, allowDot)
	if relErr != nil {
		return "", "", "", relErr
	}
	r := s.roots[0]
	candidate := filepath.Clean(filepath.Join(r, rel))
	if !fsPathWithinRoot(r, candidate) {
		return "", "", "", errors.New("path escapes workspace root")
	}
	return candidate, filepath.ToSlash(rel), r, nil
}

func (s *fsToolScope) resolvePathWithApproval(ctx context.Context, toolName, raw string, allowDot bool) (absPath string, relPath string, root string, err error) {
	trimmed := strings.TrimSpace(raw)
	if !filepath.IsAbs(trimmed) {
		return s.resolvePath(trimmed, allowDot)
	}

	absPath, relPath, root, err = s.resolvePath(trimmed, allowDot)
	if err == nil {
		return absPath, relPath, root, nil
	}
	if !strings.Contains(err.Error(), "path escapes workspace root") {
		return "", "", "", err
	}

	candidate := filepath.Clean(trimmed)
	approvalDir := externalApprovalDir(candidate, allowDot)
	if s.dirStore != nil {
		if entry := s.dirStore.Match(approvalDir); entry != nil {
			return candidate, candidate, entry.Path, nil
		}
	}
	if s.approvals == nil {
		return "", "", "", err
	}

	userID := GetUserID(ctx)
	decision, reqErr := s.approvals.RequestApproval(ctx, ApprovalRequest{
		Type:      "directory",
		Directory: approvalDir,
		Command:   strings.TrimSpace(toolName + " " + candidate),
		UserID:    userID,
	})
	if reqErr != nil {
		return "", "", "", fmt.Errorf("path approval failed: %w", reqErr)
	}

	switch decision {
	case ApprovalAllowOnce:
		return candidate, candidate, approvalDir, nil
	case ApprovalAllowAlways:
		if s.dirStore != nil {
			_ = s.dirStore.Add(approvalDir, userID)
		}
		return candidate, candidate, approvalDir, nil
	default:
		return "", "", "", fmt.Errorf("path access denied: user denied access to directory %q", approvalDir)
	}
}

func externalApprovalDir(candidate string, allowDot bool) string {
	candidate = filepath.Clean(candidate)
	if !allowDot {
		return filepath.Dir(candidate)
	}
	info, err := os.Stat(candidate)
	if err == nil && !info.IsDir() {
		return filepath.Dir(candidate)
	}
	return candidate
}

func cleanFSToolRelPath(raw string, allowDot bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.ContainsRune(trimmed, 0) {
		return "", errors.New("invalid path")
	}
	if trimmed == "" {
		if allowDot {
			return ".", nil
		}
		return "", errors.New("path is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", errors.New("path must be relative to workspace root")
	}
	clean := filepath.Clean(trimmed)
	cleanSlash := filepath.ToSlash(clean)
	if clean == "." {
		if allowDot {
			return clean, nil
		}
		return "", errors.New("path must point to a file")
	}
	if clean == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", errors.New("path cannot escape workspace root")
	}
	return clean, nil
}

func fsPathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func fsCompatKeys(key string) []string {
	switch key {
	case "path":
		return []string{"path", "file_path", "filePath"}
	case "content":
		return []string{"content", "text", "body", "value", "chunk"}
	case "old_text":
		return []string{"old_text", "oldText"}
	case "new_text":
		return []string{"new_text", "newText"}
	case "replace_all":
		return []string{"replace_all", "replaceAll"}
	case "start_line":
		return []string{"start_line", "startLine"}
	case "end_line":
		return []string{"end_line", "endLine"}
	case "max_bytes":
		return []string{"max_bytes", "maxBytes"}
	case "create_dirs":
		return []string{"create_dirs", "createDirs"}
	case "session_id":
		return []string{"session_id", "sessionId", "id"}
	case "expected_bytes":
		return []string{"expected_bytes", "expectedBytes"}
	case "max_depth":
		return []string{"max_depth", "maxDepth"}
	case "max_results":
		return []string{"max_results", "maxResults"}
	case "max_entries":
		return []string{"max_entries", "maxEntries"}
	case "case_sensitive":
		return []string{"case_sensitive", "caseSensitive"}
	case "include_hidden":
		return []string{"include_hidden", "includeHidden"}
	default:
		return []string{key}
	}
}

func fsAsString(args map[string]interface{}, key string) (string, error) {
	v, ok := firstCompatValueDeep(args, fsCompatKeys(key)...)
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return s, nil
}

func fsOptionalString(args map[string]interface{}, def, errKey string, keys ...string) (string, error) {
	v, ok := firstCompatValueDeep(args, keys...)
	if !ok {
		return def, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", errKey)
	}
	if strings.TrimSpace(s) == "" {
		return def, nil
	}
	return s, nil
}

func fsAsTextContent(args map[string]interface{}, key string) (string, error) {
	v, ok := firstCompatValueDeep(args, fsCompatKeys(key)...)
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	return fsCoerceTextContent(v)
}

func fsCoerceTextContent(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case bool:
		return strconv.FormatBool(t), nil
	case int:
		return strconv.Itoa(t), nil
	case int8:
		return strconv.FormatInt(int64(t), 10), nil
	case int16:
		return strconv.FormatInt(int64(t), 10), nil
	case int32:
		return strconv.FormatInt(int64(t), 10), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case uint:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint64:
		return strconv.FormatUint(t, 10), nil
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case map[string]interface{}, []interface{}:
		raw, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("content must be text-compatible JSON: %w", err)
		}
		return string(raw), nil
	case nil:
		return "", errors.New("content must be non-null")
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("content must be text-compatible JSON: %w", err)
		}
		return string(raw), nil
	}
}

func fsAsInt(args map[string]interface{}, key string, def int) (int, error) {
	v, ok := firstCompatValueDeep(args, fsCompatKeys(key)...)
	if !ok {
		return def, nil
	}
	switch t := v.(type) {
	case int:
		return t, nil
	case int32:
		return int(t), nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case float32:
		return int(t), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func fsAsBool(args map[string]interface{}, key string, def bool) (bool, error) {
	v, ok := firstCompatValueDeep(args, fsCompatKeys(key)...)
	if !ok {
		return def, nil
	}
	switch t := v.(type) {
	case bool:
		return t, nil
	case string:
		n := strings.TrimSpace(strings.ToLower(t))
		if n == "true" {
			return true, nil
		}
		if n == "false" {
			return false, nil
		}
	}
	return false, fmt.Errorf("%s must be a boolean", key)
}

func fsClamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func fsIsHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func fsTruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}

type EditTool struct {
	Scope       *fsToolScope
	MaxFileSize int64
}

func NewEditTool(allowedPaths []string, maxFileSize int64) *EditTool {
	if maxFileSize <= 0 || maxFileSize > maxFSToolBytes {
		maxFileSize = maxFSToolBytes
	}
	return &EditTool{Scope: newFSToolScope(allowedPaths), MaxFileSize: maxFileSize}
}

func (t *EditTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "edit",
		Description: "Make a precise in-place text replacement in a file.",
		Icon:        "file-edit",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to target file",
				},
				"old_text": map[string]interface{}{
					"type":        "string",
					"description": "Text to replace (must be non-empty)",
				},
				"new_text": map[string]interface{}{
					"type":        "string",
					"description": "Replacement text",
				},
				"replace_all": map[string]interface{}{
					"type":        "boolean",
					"description": "Replace all matches (default false)",
				},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
	}
}

func (t *EditTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, err := fsAsString(args, "path")
	if err != nil || strings.TrimSpace(path) == "" {
		return nil, errors.New("path must be a non-empty string")
	}
	oldText, err := fsAsString(args, "old_text")
	if err != nil || oldText == "" {
		return nil, errors.New("old_text must be a non-empty string")
	}
	newText, err := fsAsString(args, "new_text")
	if err != nil {
		return nil, errors.New("new_text must be a string")
	}
	replaceAll, err := fsAsBool(args, "replace_all", false)
	if err != nil {
		return nil, err
	}

	absPath, relPath, _, err := t.Scope.resolvePathWithContext(ctx, "edit", path, false)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", relPath)
	}
	if info.Size() > t.MaxFileSize {
		return nil, fmt.Errorf("file too large to edit safely: %s", relPath)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("file is not valid UTF-8 text: %s", relPath)
	}

	text := string(data)
	var replaced string
	count := 0
	if replaceAll {
		count = strings.Count(text, oldText)
		replaced = strings.ReplaceAll(text, oldText, newText)
	} else {
		if strings.Contains(text, oldText) {
			count = 1
		}
		replaced = strings.Replace(text, oldText, newText, 1)
	}
	if count == 0 {
		return nil, fmt.Errorf("target text not found in %s", relPath)
	}
	if int64(len(replaced)) > t.MaxFileSize {
		return nil, fmt.Errorf("result too large: %d bytes (max %d)", len(replaced), t.MaxFileSize)
	}
	if err := os.WriteFile(absPath, []byte(replaced), 0o644); err != nil {
		return nil, err
	}

	out, err := json.Marshal(map[string]interface{}{
		"path":         relPath,
		"replacements": count,
		"replace_all":  replaceAll,
		"success":      true,
	})
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

type GrepTool struct {
	Scope       *fsToolScope
	MaxFileSize int64
}

func NewGrepTool(allowedPaths []string, maxFileSize int64) *GrepTool {
	if maxFileSize <= 0 || maxFileSize > maxFSToolBytes {
		maxFileSize = maxFSToolBytes
	}
	return &GrepTool{Scope: newFSToolScope(allowedPaths), MaxFileSize: maxFileSize}
}

func (t *GrepTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "grep",
		Description: "Search text pattern (RE2) across files.",
		Icon:        "search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Regex pattern (RE2)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to file or directory (default .)",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Max matches to return (default 50, cap 200)",
				},
				"case_sensitive": map[string]interface{}{
					"type":        "boolean",
					"description": "Case-sensitive matching (default false)",
				},
				"include_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files and directories (default false)",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	pattern, err := fsAsString(args, "pattern")
	if err != nil || strings.TrimSpace(pattern) == "" {
		return nil, errors.New("pattern must be a non-empty string")
	}
	searchPath, err := fsOptionalString(args, ".", "path", "path", "file_path", "filePath")
	if err != nil {
		return nil, err
	}
	maxResults, err := fsAsInt(args, "max_results", 50)
	if err != nil {
		return nil, err
	}
	maxResults = fsClamp(maxResults, 1, maxFSSearchResults)
	caseSensitive, err := fsAsBool(args, "case_sensitive", false)
	if err != nil {
		return nil, err
	}
	includeHidden, err := fsAsBool(args, "include_hidden", false)
	if err != nil {
		return nil, err
	}

	regexPattern := pattern
	if !caseSensitive {
		regexPattern = "(?i)" + regexPattern
	}
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	baseAbs, baseRel, _, err := t.Scope.resolvePathWithContext(ctx, "grep", searchPath, true)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(baseAbs)
	if err != nil {
		return nil, err
	}

	type grepMatch struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
		Preview string `json:"preview"`
	}
	matches := make([]grepMatch, 0, maxResults)

	searchFile := func(path string) {
		if len(matches) >= maxResults {
			return
		}
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() || fi.Size() > t.MaxFileSize {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			return
		}
		if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
			return
		}
		_, relPath, _, relErr := t.Scope.resolvePathWithContext(ctx, "grep", path, false)
		if relErr != nil {
			return
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			loc := re.FindStringIndex(line)
			if loc == nil {
				continue
			}
			matches = append(matches, grepMatch{
				Path:    relPath,
				Line:    i + 1,
				Column:  loc[0] + 1,
				Preview: fsTruncateRunes(line, 220),
			})
			if len(matches) >= maxResults {
				return
			}
		}
	}

	if info.IsDir() {
		walkErr := filepath.WalkDir(baseAbs, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if !includeHidden && path != baseAbs && fsIsHiddenName(d.Name()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			searchFile(path)
			if len(matches) >= maxResults {
				return errFSToolWalkDone
			}
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, errFSToolWalkDone) {
			return nil, walkErr
		}
	} else {
		searchFile(baseAbs)
	}

	out, err := json.Marshal(map[string]interface{}{
		"pattern":        pattern,
		"path":           baseRel,
		"matches":        matches,
		"count":          len(matches),
		"truncated":      len(matches) >= maxResults,
		"max_results":    maxResults,
		"case_sensitive": caseSensitive,
	})
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

type FindTool struct {
	Scope *fsToolScope
}

func NewFindTool(allowedPaths []string) *FindTool {
	return &FindTool{Scope: newFSToolScope(allowedPaths)}
}

func (t *FindTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "find",
		Description: "Find files/directories by glob pattern.",
		Icon:        "folder-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern (default *)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Base path to search (default .)",
				},
				"max_depth": map[string]interface{}{
					"type":        "integer",
					"description": "Max recursion depth (default 6, cap 20)",
				},
				"include_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files and directories (default false)",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by entry type: all|file|dir (default all)",
				},
			},
		},
	}
}

func (t *FindTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	pattern, err := fsOptionalString(args, "*", "pattern", "pattern", "glob")
	if err != nil {
		return nil, err
	}
	basePath, err := fsOptionalString(args, ".", "path", "path", "file_path", "filePath")
	if err != nil {
		return nil, err
	}
	maxDepth, err := fsAsInt(args, "max_depth", defaultFSToolFindDepth)
	if err != nil {
		return nil, err
	}
	maxDepth = fsClamp(maxDepth, 0, 20)
	includeHidden, err := fsAsBool(args, "include_hidden", false)
	if err != nil {
		return nil, err
	}
	typeValue, err := fsOptionalString(args, "all", "type", "type", "file_type", "fileType")
	if err != nil {
		return nil, err
	}
	typeFilter := "all"
	n := strings.ToLower(strings.TrimSpace(typeValue))
	switch n {
	case "", "all", "file", "dir":
		if n != "" {
			typeFilter = n
		}
	default:
		return nil, errors.New("type must be one of: all, file, dir")
	}

	baseAbs, baseRel, _, err := t.Scope.resolvePathWithContext(ctx, "find", basePath, true)
	if err != nil {
		return nil, err
	}
	baseInfo, err := os.Stat(baseAbs)
	if err != nil {
		return nil, err
	}
	if !baseInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", baseRel)
	}

	type findEntry struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Size int64  `json:"size,omitempty"`
	}
	entries := make([]findEntry, 0, 256)
	truncated := false
	patternHasSlash := strings.Contains(pattern, "/")

	matchPattern := func(relPath, name string) bool {
		target := name
		if patternHasSlash {
			target = relPath
		}
		ok, matchErr := filepath.Match(pattern, target)
		if matchErr != nil {
			return false
		}
		return ok
	}

	walkErr := filepath.WalkDir(baseAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == baseAbs {
			return nil
		}
		rel, relErr := filepath.Rel(baseAbs, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		depth := strings.Count(rel, "/") + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !includeHidden && fsIsHiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entryType := "file"
		if d.IsDir() {
			entryType = "dir"
		}
		if typeFilter != "all" && typeFilter != entryType {
			return nil
		}
		if !matchPattern(rel, d.Name()) {
			return nil
		}
		item := findEntry{Path: rel, Type: entryType}
		if !d.IsDir() {
			if fi, fiErr := d.Info(); fiErr == nil {
				item.Size = fi.Size()
			}
		}
		entries = append(entries, item)
		if len(entries) >= maxFSToolEntries {
			truncated = true
			return errFSToolWalkDone
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errFSToolWalkDone) {
		return nil, walkErr
	}

	out, err := json.Marshal(map[string]interface{}{
		"base_path":      baseRel,
		"pattern":        pattern,
		"entries":        entries,
		"count":          len(entries),
		"truncated":      truncated,
		"max_depth":      maxDepth,
		"include_hidden": includeHidden,
		"type":           typeFilter,
	})
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

type LsTool struct {
	Scope *fsToolScope
}

func NewLsTool(allowedPaths []string) *LsTool {
	return &LsTool{Scope: newFSToolScope(allowedPaths)}
}

func (t *LsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "ls",
		Description: "List files and directories.",
		Icon:        "list",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to list (default .)",
				},
				"max_depth": map[string]interface{}{
					"type":        "integer",
					"description": "Max recursion depth (default 1, cap 20)",
				},
				"include_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files and directories (default false)",
				},
				"max_entries": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum entries returned (default 200, cap 5000)",
				},
			},
		},
	}
}

func (t *LsTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	listPath, err := fsOptionalString(args, ".", "path", "path", "file_path", "filePath")
	if err != nil {
		return nil, err
	}
	maxDepth, err := fsAsInt(args, "max_depth", defaultFSToolLsDepth)
	if err != nil {
		return nil, err
	}
	maxDepth = fsClamp(maxDepth, 0, 20)
	includeHidden, err := fsAsBool(args, "include_hidden", false)
	if err != nil {
		return nil, err
	}
	maxEntries, err := fsAsInt(args, "max_entries", defaultFSToolLsEntries)
	if err != nil {
		return nil, err
	}
	maxEntries = fsClamp(maxEntries, 1, maxFSToolEntries)

	baseAbs, baseRel, _, err := t.Scope.resolvePathWithContext(ctx, "ls", listPath, true)
	if err != nil {
		return nil, err
	}
	baseInfo, err := os.Stat(baseAbs)
	if err != nil {
		return nil, err
	}
	if !baseInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", baseRel)
	}

	type listEntry struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Size int64  `json:"size,omitempty"`
	}
	entries := make([]listEntry, 0, 256)
	truncated := false

	walkErr := filepath.WalkDir(baseAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == baseAbs {
			return nil
		}
		rel, relErr := filepath.Rel(baseAbs, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		depth := strings.Count(rel, "/") + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !includeHidden && fsIsHiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		item := listEntry{Path: rel, Type: "file"}
		if d.IsDir() {
			item.Type = "dir"
		} else if fi, fiErr := d.Info(); fiErr == nil {
			item.Size = fi.Size()
		}
		entries = append(entries, item)
		if len(entries) >= maxEntries {
			truncated = true
			return errFSToolWalkDone
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errFSToolWalkDone) {
		return nil, walkErr
	}

	out, err := json.Marshal(map[string]interface{}{
		"base_path":      baseRel,
		"entries":        entries,
		"count":          len(entries),
		"truncated":      truncated,
		"max_depth":      maxDepth,
		"max_entries":    maxEntries,
		"include_hidden": includeHidden,
	})
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

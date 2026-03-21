package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type grepMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview"`
}

type findEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

type grepRequest struct {
	Pattern       string
	SearchPath    string
	MaxResults    int
	CaseSensitive bool
	IncludeHidden bool
	Compiled      *regexp.Regexp
}

type findRequest struct {
	Pattern       string
	BasePath      string
	MaxDepth      int
	IncludeHidden bool
	TypeFilter    string
}

func (t *GrepTool) toolName() string {
	if t == nil || strings.TrimSpace(t.Name) == "" {
		return "grep"
	}
	return strings.TrimSpace(t.Name)
}

func (t *GrepTool) toolDescription() string {
	if t == nil || strings.TrimSpace(t.Description) == "" {
		return "Search text pattern (RE2) across files."
	}
	return strings.TrimSpace(t.Description)
}

func (t *GrepTool) executeGrep(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	req, err := parseGrepRequest(args)
	if err != nil {
		return nil, err
	}
	baseAbs, baseRel, _, err := t.Scope.resolvePathWithContext(ctx, t.toolName(), req.SearchPath, true)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(baseAbs)
	if err != nil {
		return nil, err
	}

	fallbackReason := ""
	if t != nil && t.Ripgrep != nil {
		if payload, err := t.executeRipgrepGrep(ctx, req, baseAbs, baseRel, info); err == nil {
			return marshalFSToolPayload(payload)
		} else if err != nil {
			fallbackReason = err.Error()
		}
	}

	payload, err := t.executeBuiltinGrep(ctx, req, baseAbs, baseRel, info)
	if err != nil {
		return nil, err
	}
	attachFSBackendFields(payload, "builtin", "builtin", fallbackReason)
	return marshalFSToolPayload(payload)
}

func parseGrepRequest(args map[string]interface{}) (grepRequest, error) {
	pattern, err := fsAsString(args, "pattern")
	if err != nil || strings.TrimSpace(pattern) == "" {
		return grepRequest{}, errors.New("pattern must be a non-empty string")
	}
	searchPath, err := fsOptionalString(args, ".", "path", "path", "file_path", "filePath")
	if err != nil {
		return grepRequest{}, err
	}
	maxResults, err := fsAsInt(args, "max_results", 50)
	if err != nil {
		return grepRequest{}, err
	}
	maxResults = fsClamp(maxResults, 1, maxFSSearchResults)
	caseSensitive, err := fsAsBool(args, "case_sensitive", false)
	if err != nil {
		return grepRequest{}, err
	}
	includeHidden, err := fsAsBool(args, "include_hidden", false)
	if err != nil {
		return grepRequest{}, err
	}

	regexPattern := pattern
	if !caseSensitive {
		regexPattern = "(?i)" + regexPattern
	}
	compiled, err := regexp.Compile(regexPattern)
	if err != nil {
		return grepRequest{}, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return grepRequest{
		Pattern:       pattern,
		SearchPath:    searchPath,
		MaxResults:    maxResults,
		CaseSensitive: caseSensitive,
		IncludeHidden: includeHidden,
		Compiled:      compiled,
	}, nil
}

func (t *GrepTool) executeBuiltinGrep(ctx context.Context, req grepRequest, baseAbs, baseRel string, info os.FileInfo) (map[string]interface{}, error) {
	matches := make([]grepMatch, 0, req.MaxResults)

	searchFile := func(path string) {
		if len(matches) >= req.MaxResults {
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
		_, relPath, _, relErr := t.Scope.resolvePathWithContext(ctx, t.toolName(), path, false)
		if relErr != nil {
			return
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			loc := req.Compiled.FindStringIndex(line)
			if loc == nil {
				continue
			}
			matches = append(matches, grepMatch{
				Path:    relPath,
				Line:    i + 1,
				Column:  loc[0] + 1,
				Preview: fsTruncateRunes(line, 220),
			})
			if len(matches) >= req.MaxResults {
				return
			}
		}
	}

	if info.IsDir() {
		walkErr := filepath.WalkDir(baseAbs, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if !req.IncludeHidden && path != baseAbs && fsIsHiddenName(d.Name()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			searchFile(path)
			if len(matches) >= req.MaxResults {
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

	return map[string]interface{}{
		"pattern":        req.Pattern,
		"path":           baseRel,
		"matches":        matches,
		"count":          len(matches),
		"truncated":      len(matches) >= req.MaxResults,
		"max_results":    req.MaxResults,
		"case_sensitive": req.CaseSensitive,
	}, nil
}

func (t *GrepTool) executeRipgrepGrep(ctx context.Context, req grepRequest, baseAbs, baseRel string, info os.FileInfo) (map[string]interface{}, error) {
	bin, err := t.Ripgrep.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	workDir := baseAbs
	searchTarget := "."
	if !info.IsDir() {
		workDir = filepath.Dir(baseAbs)
		searchTarget = filepath.Base(baseAbs)
	}

	args := []string{
		"--json",
		"--line-number",
		"--column",
		"--color=never",
		"--no-ignore",
	}
	if req.IncludeHidden {
		args = append(args, "--hidden")
	}
	if req.CaseSensitive {
		args = append(args, "--case-sensitive")
	} else {
		args = append(args, "--ignore-case")
	}
	args = append(args, "--regexp", req.Pattern, searchTarget)

	out, exitCode, err := t.ripgrepExec(ctx, bin.Path, args, workDir)
	if err != nil && exitCode != 1 {
		return nil, fmt.Errorf("ripgrep execution failed: %w", err)
	}
	if exitCode != 0 && exitCode != 1 {
		return nil, fmt.Errorf("ripgrep execution failed with exit code %d", exitCode)
	}

	matches, truncated, err := t.parseRipgrepMatches(ctx, out, workDir, req, info.IsDir())
	if err != nil {
		return nil, fmt.Errorf("ripgrep output parse failed: %w", err)
	}

	payload := map[string]interface{}{
		"pattern":        req.Pattern,
		"path":           baseRel,
		"matches":        matches,
		"count":          len(matches),
		"truncated":      truncated,
		"max_results":    req.MaxResults,
		"case_sensitive": req.CaseSensitive,
	}
	attachFSBackendFields(payload, "ripgrep", bin.Source, "")
	return payload, nil
}

func (t *GrepTool) parseRipgrepMatches(ctx context.Context, out []byte, workDir string, req grepRequest, directorySearch bool) ([]grepMatch, bool, error) {
	type submatch struct {
		Start int `json:"start"`
	}
	type rgText struct {
		Text string `json:"text"`
	}
	type rgEvent struct {
		Type string `json:"type"`
		Data struct {
			Path       rgText     `json:"path"`
			Lines      rgText     `json:"lines"`
			LineNumber int        `json:"line_number"`
			Submatches []submatch `json:"submatches"`
		} `json:"data"`
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), maxFSToolBytes+1024)

	matches := make([]grepMatch, 0, req.MaxResults)
	truncated := false
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event rgEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, false, err
		}
		if event.Type != "match" {
			continue
		}
		relResultPath := strings.TrimSpace(event.Data.Path.Text)
		if relResultPath == "" {
			continue
		}
		normalizedRel := filepath.ToSlash(strings.ReplaceAll(relResultPath, "\\", "/"))
		if !req.IncludeHidden && directorySearch && fsPathHasHiddenSegment(normalizedRel) {
			continue
		}
		absPath := filepath.Clean(filepath.Join(workDir, filepath.FromSlash(normalizedRel)))
		fileInfo, err := os.Stat(absPath)
		if err != nil || fileInfo.IsDir() || fileInfo.Size() > t.MaxFileSize {
			continue
		}
		_, workspaceRel, _, err := t.Scope.resolvePathWithContext(ctx, t.toolName(), absPath, false)
		if err != nil {
			continue
		}
		column := 1
		if len(event.Data.Submatches) > 0 {
			column = event.Data.Submatches[0].Start + 1
		}
		preview := strings.TrimRight(event.Data.Lines.Text, "\r\n")
		if len(matches) < req.MaxResults {
			matches = append(matches, grepMatch{
				Path:    workspaceRel,
				Line:    event.Data.LineNumber,
				Column:  column,
				Preview: fsTruncateRunes(preview, 220),
			})
			continue
		}
		truncated = true
		break
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	return matches, truncated, nil
}

func (t *FindTool) executeFind(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	req, err := parseFindRequest(args)
	if err != nil {
		return nil, err
	}
	baseAbs, baseRel, _, err := t.Scope.resolvePathWithContext(ctx, "find", req.BasePath, true)
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

	fallbackReason := ""
	if t != nil && t.Ripgrep != nil && req.TypeFilter == "file" && canUseRipgrepFindGlob(req.Pattern) {
		if payload, err := t.executeRipgrepFind(ctx, req, baseAbs, baseRel); err == nil {
			return marshalFSToolPayload(payload)
		} else if err != nil {
			fallbackReason = err.Error()
		}
	}

	payload, err := t.executeBuiltinFind(req, baseAbs, baseRel)
	if err != nil {
		return nil, err
	}
	attachFSBackendFields(payload, "builtin", "builtin", fallbackReason)
	return marshalFSToolPayload(payload)
}

func parseFindRequest(args map[string]interface{}) (findRequest, error) {
	pattern, err := fsOptionalString(args, "*", "pattern", "pattern", "glob")
	if err != nil {
		return findRequest{}, err
	}
	basePath, err := fsOptionalString(args, ".", "path", "path", "file_path", "filePath")
	if err != nil {
		return findRequest{}, err
	}
	maxDepth, err := fsAsInt(args, "max_depth", defaultFSToolFindDepth)
	if err != nil {
		return findRequest{}, err
	}
	maxDepth = fsClamp(maxDepth, 0, 20)
	includeHidden, err := fsAsBool(args, "include_hidden", false)
	if err != nil {
		return findRequest{}, err
	}
	typeValue, err := fsOptionalString(args, "all", "type", "type", "file_type", "fileType")
	if err != nil {
		return findRequest{}, err
	}
	typeFilter := "all"
	switch n := strings.ToLower(strings.TrimSpace(typeValue)); n {
	case "", "all", "file", "dir":
		if n != "" {
			typeFilter = n
		}
	default:
		return findRequest{}, errors.New("type must be one of: all, file, dir")
	}

	return findRequest{
		Pattern:       pattern,
		BasePath:      basePath,
		MaxDepth:      maxDepth,
		IncludeHidden: includeHidden,
		TypeFilter:    typeFilter,
	}, nil
}

func (t *FindTool) executeBuiltinFind(req findRequest, baseAbs, baseRel string) (map[string]interface{}, error) {
	entries := make([]findEntry, 0, 256)
	truncated := false
	matchPattern := makeFindPatternMatcher(req.Pattern)

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
		if depth > req.MaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !req.IncludeHidden && fsIsHiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entryType := "file"
		if d.IsDir() {
			entryType = "dir"
		}
		if req.TypeFilter != "all" && req.TypeFilter != entryType {
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

	return map[string]interface{}{
		"base_path":      baseRel,
		"pattern":        req.Pattern,
		"entries":        entries,
		"count":          len(entries),
		"truncated":      truncated,
		"max_depth":      req.MaxDepth,
		"include_hidden": req.IncludeHidden,
		"type":           req.TypeFilter,
	}, nil
}

func (t *FindTool) executeRipgrepFind(ctx context.Context, req findRequest, baseAbs, baseRel string) (map[string]interface{}, error) {
	bin, err := t.Ripgrep.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	args := []string{"--files", "--null", "--no-ignore"}
	if req.IncludeHidden {
		args = append(args, "--hidden")
	}
	if req.Pattern != "*" {
		args = append(args, "--glob", req.Pattern)
	}
	out, exitCode, err := t.ripgrepExec(ctx, bin.Path, args, baseAbs)
	if err != nil && exitCode != 1 {
		return nil, fmt.Errorf("ripgrep file listing failed: %w", err)
	}
	if exitCode != 0 && exitCode != 1 {
		return nil, fmt.Errorf("ripgrep file listing failed with exit code %d", exitCode)
	}

	entries := make([]findEntry, 0, 256)
	truncated := false
	matchPattern := makeFindPatternMatcher(req.Pattern)
	for _, raw := range bytes.Split(out, []byte{0}) {
		rel := strings.TrimSpace(string(raw))
		if rel == "" {
			continue
		}
		rel = filepath.ToSlash(strings.ReplaceAll(rel, "\\", "/"))
		if !req.IncludeHidden && fsPathHasHiddenSegment(rel) {
			continue
		}
		depth := strings.Count(rel, "/") + 1
		if depth > req.MaxDepth {
			continue
		}
		name := filepath.Base(filepath.FromSlash(rel))
		if !matchPattern(rel, name) {
			continue
		}

		absPath := filepath.Clean(filepath.Join(baseAbs, filepath.FromSlash(rel)))
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			continue
		}
		entries = append(entries, findEntry{
			Path: rel,
			Type: "file",
			Size: info.Size(),
		})
		if len(entries) >= maxFSToolEntries {
			truncated = true
			break
		}
	}

	payload := map[string]interface{}{
		"base_path":      baseRel,
		"pattern":        req.Pattern,
		"entries":        entries,
		"count":          len(entries),
		"truncated":      truncated,
		"max_depth":      req.MaxDepth,
		"include_hidden": req.IncludeHidden,
		"type":           req.TypeFilter,
	}
	attachFSBackendFields(payload, "ripgrep", bin.Source, "")
	return payload, nil
}

func attachFSBackendFields(payload map[string]interface{}, backend, source, fallbackReason string) {
	payload["backend"] = backend
	payload["backend_source"] = source
	payload["fallback_reason"] = fallbackReason
}

func marshalFSToolPayload(payload map[string]interface{}) (interface{}, error) {
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

func makeFindPatternMatcher(pattern string) func(relPath, name string) bool {
	patternHasSlash := strings.Contains(pattern, "/")
	return func(relPath, name string) bool {
		target := name
		if patternHasSlash {
			target = relPath
		}
		ok, err := filepath.Match(pattern, target)
		if err != nil {
			return false
		}
		return ok
	}
}

func fsPathHasHiddenSegment(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if fsIsHiddenName(part) {
			return true
		}
	}
	return false
}

func canUseRipgrepFindGlob(pattern string) bool {
	if strings.TrimSpace(pattern) == "" || strings.HasPrefix(pattern, "!") {
		return false
	}
	for _, r := range pattern {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case strings.ContainsRune("._-/*?", r):
		default:
			return false
		}
	}
	return true
}

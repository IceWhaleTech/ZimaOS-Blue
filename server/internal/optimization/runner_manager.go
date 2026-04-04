package optimization

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
	"slices"
)

const DefaultGoVersion = "1.24.0"

type Platform struct {
	GOOS   string
	GOARCH string
}

type GitHubRepo struct {
	Owner        string
	Name         string
	CanonicalURL string
	Slug         string
}

type GoArchive struct {
	Version  string
	FileName string
	URL      string
	SHA256   string
}

type ManagedLayout struct {
	RootDir         string
	ReposDir        string
	GoToolchainsDir string
	GoCacheDir      string
	GoModCacheDir   string
	BinDir          string
	StatusPath      string
}

type PrepareRequest struct {
	RepoURL        string   `json:"repo_url,omitempty"`
	Ref            string   `json:"ref,omitempty"`
	RequestedParts []string `json:"requested_parts,omitempty"`
}

type Status struct {
	Enabled                 bool       `json:"enabled"`
	RepoURL                 string     `json:"repo_url,omitempty"`
	ResolvedRef             string     `json:"resolved_ref,omitempty"`
	ResolvedCommit          string     `json:"resolved_commit,omitempty"`
	RequiredGoVersion       string     `json:"required_go_version,omitempty"`
	InstalledGoVersion      string     `json:"installed_go_version,omitempty"`
	ToolchainReady          bool       `json:"toolchain_ready"`
	BinaryReady             bool       `json:"binary_ready"`
	BinaryPath              string     `json:"binary_path,omitempty"`
	BinarySHA256            string     `json:"binary_sha256,omitempty"`
	ManifestPath            string     `json:"manifest_path,omitempty"`
	SupportedParts          []string   `json:"supported_parts"`
	OptimizedParts          []string   `json:"optimized_parts"`
	PrimaryPart             string     `json:"primary_part,omitempty"`
	SourceOptimizationRunID string     `json:"source_optimization_run_id,omitempty"`
	SourceEvalRunID         string     `json:"source_eval_run_id,omitempty"`
	LastPrepareAt           *time.Time `json:"last_prepare_at,omitempty"`
	LastPrepareState        string     `json:"last_prepare_state,omitempty"`
	LastError               string     `json:"last_error,omitempty"`
	LastOptimizationRunID   string     `json:"last_optimization_run_id,omitempty"`
	LastOptimizationAt      *time.Time `json:"last_optimization_at,omitempty"`
	LastOptimizationState   string     `json:"last_optimization_state,omitempty"`
	LastOptimizationSummary string     `json:"last_optimization_summary,omitempty"`
}

type RunnerExecutionResult struct {
	Protocol     string                  `json:"protocol,omitempty"`
	SessionID    string                  `json:"session_id,omitempty"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	ResponseText string                  `json:"response_text,omitempty"`
	Stderr       string                  `json:"stderr,omitempty"`
	Transcript   []RunnerTranscriptEntry `json:"transcript,omitempty"`
}

type RunnerTranscriptEntry struct {
	Direction string `json:"direction,omitempty"`
	Method    string `json:"method,omitempty"`
	ID        string `json:"id,omitempty"`
	Text      string `json:"text,omitempty"`
}

type OptimizationRunRecord map[string]interface{}

type Manager struct {
	mu sync.Mutex

	layout       ManagedLayout
	status       Status
	httpClient   *http.Client
	indexURL     string
	downloadBase string
}

func NormalizeGitHubRepo(input string) (GitHubRepo, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return GitHubRepo{}, fmt.Errorf("repo url is required")
	}
	trimmed := strings.TrimSuffix(raw, ".git")
	if !strings.Contains(trimmed, "://") {
		if strings.Contains(trimmed, "@") || strings.Contains(trimmed, ":") {
			return GitHubRepo{}, fmt.Errorf("only public https github.com repositories are supported")
		}
		parts := strings.Split(trimmed, "/")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return GitHubRepo{}, fmt.Errorf("repo must be <owner>/<repo> or https://github.com/<owner>/<repo>")
		}
		owner := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		return GitHubRepo{
			Owner:        owner,
			Name:         name,
			CanonicalURL: "https://github.com/" + owner + "/" + name,
			Slug:         owner + "__" + name,
		}, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return GitHubRepo{}, err
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Host, "github.com") {
		return GitHubRepo{}, fmt.Errorf("only public https github.com repositories are supported")
	}
	path := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return GitHubRepo{}, fmt.Errorf("github repo url must point to /<owner>/<repo>")
	}
	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	return GitHubRepo{
		Owner:        owner,
		Name:         name,
		CanonicalURL: "https://github.com/" + owner + "/" + name,
		Slug:         owner + "__" + name,
	}, nil
}

func ListGitHubRepoTags(ctx context.Context, input string) ([]string, error) {
	repo, err := NormalizeGitHubRepo(input)
	if err != nil {
		return nil, err
	}
	output, err := runCommand(ctx, "", "git", "ls-remote", "--refs", "--tags", repo.CanonicalURL)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	tags := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		ref := strings.TrimSpace(fields[1])
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		tag := strings.TrimSpace(strings.TrimPrefix(ref, "refs/tags/"))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	slices.SortFunc(tags, func(a, b string) int {
		return cmp.Compare(b, a)
	})
	return tags, nil
}

func DetectRequiredGoVersion(goMod []byte) string {
	if len(goMod) == 0 {
		return DefaultGoVersion
	}
	if parsed, err := modfile.Parse("go.mod", goMod, nil); err == nil {
		if parsed.Go != nil {
			version := strings.TrimSpace(parsed.Go.Version)
			if version != "" {
				return version
			}
		}
	}
	re := regexp.MustCompile(`(?m)^\s*go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)\s*$`)
	matches := re.FindSubmatch(goMod)
	if len(matches) == 2 {
		return strings.TrimSpace(string(matches[1]))
	}
	return DefaultGoVersion
}

func GoToolchainArchive(version string, platform Platform) (GoArchive, error) {
	version = normalizeGoVersion(version)
	if platform.GOOS == "" || platform.GOARCH == "" {
		return GoArchive{}, fmt.Errorf("goos and goarch are required")
	}
	switch platform.GOOS {
	case "darwin", "linux", "windows":
	default:
		return GoArchive{}, fmt.Errorf("unsupported goos %q", platform.GOOS)
	}
	switch platform.GOARCH {
	case "amd64", "arm64":
	default:
		return GoArchive{}, fmt.Errorf("unsupported goarch %q", platform.GOARCH)
	}
	ext := ".tar.gz"
	if platform.GOOS == "windows" {
		ext = ".zip"
	}
	fileName := fmt.Sprintf("go%s.%s-%s%s", version, platform.GOOS, platform.GOARCH, ext)
	return GoArchive{
		Version:  version,
		FileName: fileName,
		URL:      "https://go.dev/dl/" + fileName,
	}, nil
}

func NewManagedLayout(root string) ManagedLayout {
	root = filepath.Clean(strings.TrimSpace(root))
	return ManagedLayout{
		RootDir:         root,
		ReposDir:        filepath.Join(root, "repos"),
		GoToolchainsDir: filepath.Join(root, "toolchains", "go"),
		GoCacheDir:      filepath.Join(root, "gocache"),
		GoModCacheDir:   filepath.Join(root, "gomodcache"),
		BinDir:          filepath.Join(root, "bin"),
		StatusPath:      filepath.Join(root, "status.json"),
	}
}

func RunnerBinaryName(goos string) string {
	if strings.EqualFold(goos, "windows") {
		return "agentcore-runner.exe"
	}
	return "agentcore-runner"
}

func NewDefaultManager() (*Manager, error) {
	root, err := defaultCacheRoot()
	if err != nil {
		return nil, err
	}
	return NewManager(root)
}

func NewManager(root string) (*Manager, error) {
	layout := NewManagedLayout(root)
	for _, dir := range []string{
		layout.RootDir,
		layout.ReposDir,
		layout.GoToolchainsDir,
		layout.GoCacheDir,
		layout.GoModCacheDir,
		layout.BinDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	manager := &Manager{
		layout:       layout,
		httpClient:   &http.Client{Timeout: 5 * time.Minute},
		indexURL:     "https://go.dev/dl/?mode=json&include=all",
		downloadBase: "https://go.dev/dl/",
	}
	manager.loadStatus()
	return manager, nil
}

func (m *Manager) GetStatus(_ context.Context) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := cloneStatus(m.status)
	enrichStatusWithLastOptimizationSummary(m.layout, &status)
	NormalizeStatusEvolvableParts(&status)
	return status
}

func (m *Manager) Prepare(ctx context.Context, req PrepareRequest) (Status, error) {
	requestedParts, err := NormalizeRequestedEvolvableParts(req.RequestedParts)
	if err != nil {
		return Status{}, err
	}
	req.RequestedParts = requestedParts
	repo, err := NormalizeGitHubRepo(req.RepoURL)
	if err != nil {
		return m.failStatus(err)
	}
	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		ref = "HEAD"
	}

	m.mu.Lock()
	now := time.Now().UTC()
	m.status.RepoURL = repo.CanonicalURL
	m.status.ResolvedRef = ref
	m.status.LastPrepareAt = &now
	m.status.LastPrepareState = "preparing"
	m.status.LastError = ""
	_ = m.persistLocked()
	m.mu.Unlock()

	repoDir, resolvedCommit, err := m.prepareRepo(ctx, repo, ref)
	if err != nil {
		return m.failStatus(err)
	}

	goModBytes, err := os.ReadFile(filepath.Join(repoDir, "go.mod"))
	if err != nil {
		return m.failStatus(err)
	}
	requiredGoVersion := DetectRequiredGoVersion(goModBytes)

	toolchainRoot, err := m.ensureGoToolchain(ctx, requiredGoVersion, Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	if err != nil {
		return m.failStatus(err)
	}

	binaryPath, binarySHA, err := m.buildRunnerBinary(ctx, repoDir, repo, resolvedCommit, toolchainRoot)
	if err != nil {
		return m.failStatus(err)
	}
	optimizedParts := append([]string{}, req.RequestedParts...)
	manifestPath, err := writeRunnerArtifactManifest(binaryPath, RunnerArtifactManifest{
		SchemaVersion:         RunnerArtifactManifestSchemaVersion,
		BinarySHA256:          binarySHA,
		RepoURL:               repo.CanonicalURL,
		Ref:                   ref,
		Commit:                resolvedCommit,
		SupportedParts:        DefaultSupportedEvolvableParts(),
		OptimizedParts:        optimizedParts,
		PrimaryPart:           firstStringFromSlice(optimizedParts),
		SourceOptimizationRun: "",
		SourceEvalRun:         "",
		BuiltAt:               time.Now().UTC(),
	})
	if err != nil {
		return m.failStatus(err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.RepoURL = repo.CanonicalURL
	m.status.ResolvedRef = ref
	m.status.ResolvedCommit = resolvedCommit
	m.status.RequiredGoVersion = requiredGoVersion
	m.status.InstalledGoVersion = requiredGoVersion
	m.status.ToolchainReady = true
	m.status.BinaryReady = true
	m.status.BinaryPath = binaryPath
	m.status.BinarySHA256 = binarySHA
	m.status.LastPrepareState = "ready"
	m.status.LastError = ""
	m.status.ManifestPath = manifestPath
	m.status.SupportedParts = append([]string{}, DefaultSupportedEvolvableParts()...)
	m.status.OptimizedParts = optimizedParts
	m.status.PrimaryPart = firstStringFromSlice(optimizedParts)
	m.status.SourceOptimizationRunID = ""
	m.status.SourceEvalRunID = ""
	_ = m.persistLocked()
	status := cloneStatus(m.status)
	NormalizeStatusEvolvableParts(&status)
	return status, nil
}

func (m *Manager) SetLastOptimizationRunID(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastOptimizationRunID = strings.TrimSpace(id)
	return m.persistLocked()
}

func (m *Manager) RecordOptimizationEvent(id string, payload interface{}) error {
	if m == nil {
		return fmt.Errorf("manager is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	dir := filepath.Join(m.layout.RootDir, "optimization-runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	normalizedPayload := payload
	switch typed := payload.(type) {
	case OptimizationRunRecord:
		normalizedPayload = NormalizeOptimizationRunRecordEvolvableParts(typed)
	case map[string]interface{}:
		normalizedPayload = NormalizeOptimizationRunRecordEvolvableParts(OptimizationRunRecord(typed))
	}
	data, err := json.MarshalIndent(normalizedPayload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644)
}

func (m *Manager) GetLastOptimizationRun(_ context.Context) (OptimizationRunRecord, error) {
	if m == nil {
		return nil, fmt.Errorf("manager is nil")
	}
	m.mu.Lock()
	status := cloneStatus(m.status)
	layout := m.layout
	m.mu.Unlock()
	runID := strings.TrimSpace(status.LastOptimizationRunID)
	if runID == "" {
		return nil, nil
	}
	return readOptimizationRunRecord(layout, runID)
}

func (m *Manager) GetOptimizationRun(_ context.Context, id string) (OptimizationRunRecord, error) {
	if m == nil {
		return nil, fmt.Errorf("manager is nil")
	}
	return readOptimizationRunRecord(m.layout, strings.TrimSpace(id))
}

func (m *Manager) ExecutePreparedRunnerACP(ctx context.Context, prompt string) (RunnerExecutionResult, error) {
	if m == nil {
		return RunnerExecutionResult{}, fmt.Errorf("manager is nil")
	}
	status := m.GetStatus(ctx)
	binaryPath := strings.TrimSpace(status.BinaryPath)
	if !status.BinaryReady || binaryPath == "" {
		return RunnerExecutionResult{}, fmt.Errorf("runner binary is not ready")
	}
	if err := validatePreparedRunnerBinary(binaryPath, status.BinarySHA256); err != nil {
		return RunnerExecutionResult{}, err
	}
	cmd := exec.CommandContext(ctx, binaryPath, "acp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return RunnerExecutionResult{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return RunnerExecutionResult{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return RunnerExecutionResult{}, err
	}

	result := RunnerExecutionResult{Protocol: "acp"}
	reader := bufio.NewReader(stdout)
	requestID := 0
	appendTranscript := func(direction string, payload map[string]interface{}) {
		result.Transcript = append(result.Transcript, runnerTranscriptEntry(direction, payload))
		if text := strings.TrimSpace(runnerTranscriptText(payload)); text != "" && direction == "in" {
			if result.ResponseText == "" {
				result.ResponseText = text
				return
			}
			result.ResponseText += "\n" + text
		}
	}
	sendRequest := func(method string, params map[string]interface{}) (string, error) {
		requestID++
		id := fmt.Sprintf("%d", requestID)
		payload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  method,
		}
		if params != nil {
			payload["params"] = params
		}
		appendTranscript("out", payload)
		data, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		if _, err := stdin.Write(append(data, '\n')); err != nil {
			return "", err
		}
		return id, nil
	}
	readUntilResponse := func(expectedID string) (map[string]interface{}, error) {
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, err
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(bytes.TrimSpace(line), &payload); err != nil {
				return nil, err
			}
			appendTranscript("in", payload)
			if errMessage := jsonRPCErrorMessage(payload); errMessage != "" {
				return nil, fmt.Errorf("runner returned error: %s", errMessage)
			}
			if strings.TrimSpace(fmt.Sprint(payload["id"])) == expectedID {
				return payload, nil
			}
		}
	}

	initializeID, err := sendRequest("initialize", map[string]interface{}{"protocolVersion": 1})
	if err != nil {
		return result, err
	}
	if _, err := readUntilResponse(initializeID); err != nil {
		return finishRunnerExecution(cmd, stdin, &stderr, result, err)
	}

	sessionIDRequest, err := sendRequest("session/new", map[string]interface{}{"cwd": m.layout.RootDir})
	if err != nil {
		return finishRunnerExecution(cmd, stdin, &stderr, result, err)
	}
	sessionResp, err := readUntilResponse(sessionIDRequest)
	if err != nil {
		return finishRunnerExecution(cmd, stdin, &stderr, result, err)
	}
	if sessionResult, _ := sessionResp["result"].(map[string]interface{}); sessionResult != nil {
		result.SessionID = strings.TrimSpace(fmt.Sprint(sessionResult["sessionId"]))
	}
	if result.SessionID == "" {
		return finishRunnerExecution(cmd, stdin, &stderr, result, fmt.Errorf("runner session/new returned empty session id"))
	}

	promptID, err := sendRequest("session/prompt", map[string]interface{}{
		"sessionId": result.SessionID,
		"prompt": []map[string]interface{}{
			{
				"type": "text",
				"text": strings.TrimSpace(prompt),
			},
		},
	})
	if err != nil {
		return finishRunnerExecution(cmd, stdin, &stderr, result, err)
	}
	promptResp, err := readUntilResponse(promptID)
	if err != nil {
		return finishRunnerExecution(cmd, stdin, &stderr, result, err)
	}
	if promptResult, _ := promptResp["result"].(map[string]interface{}); promptResult != nil {
		result.StopReason = strings.TrimSpace(fmt.Sprint(promptResult["stopReason"]))
	}
	if strings.TrimSpace(result.StopReason) == "" {
		return finishRunnerExecution(cmd, stdin, &stderr, result, fmt.Errorf("runner prompt response missing stop reason"))
	}
	return finishRunnerExecution(cmd, stdin, &stderr, result, nil)
}

func defaultCacheRoot() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "zimaos-blue", "optimization", "agentcore-runner"), nil
}

func (m *Manager) prepareRepo(ctx context.Context, repo GitHubRepo, ref string) (string, string, error) {
	commit, err := resolveGitCommit(ctx, repo.CanonicalURL, ref)
	if err != nil {
		return "", "", err
	}
	finalDir := filepath.Join(m.layout.ReposDir, repo.Slug, commit)
	if stat, err := os.Stat(finalDir); err == nil && stat.IsDir() {
		return finalDir, commit, nil
	}
	parentDir := filepath.Join(m.layout.ReposDir, repo.Slug)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", "", err
	}
	tempDir, err := os.MkdirTemp(parentDir, "clone-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tempDir)

	if _, err := runCommand(ctx, tempDir, "git", "clone", "--filter=blob:none", "--no-checkout", repo.CanonicalURL, "repo"); err != nil {
		return "", "", err
	}
	repoCloneDir := filepath.Join(tempDir, "repo")
	if _, err := runCommand(ctx, repoCloneDir, "git", "fetch", "--depth", "1", "origin", commit); err != nil {
		return "", "", err
	}
	if _, err := runCommand(ctx, repoCloneDir, "git", "checkout", "--detach", "FETCH_HEAD"); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return "", "", err
	}
	if err := os.Rename(repoCloneDir, finalDir); err != nil {
		if stat, statErr := os.Stat(finalDir); statErr == nil && stat.IsDir() {
			return finalDir, commit, nil
		}
		return "", "", err
	}
	return finalDir, commit, nil
}

func resolveGitCommit(ctx context.Context, repoURL string, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	output, err := runCommand(ctx, "", "git", "ls-remote", repoURL, ref)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 1 && len(fields[0]) == 40 {
				return fields[0], nil
			}
		}
	}
	if isFullSHA(ref) {
		return ref, nil
	}
	return "", fmt.Errorf("failed to resolve git ref %q", ref)
}

func (m *Manager) ensureGoToolchain(ctx context.Context, version string, platform Platform) (string, error) {
	version = normalizeGoVersion(version)
	finalBase := filepath.Join(m.layout.GoToolchainsDir, version, platform.GOOS+"-"+platform.GOARCH)
	finalRoot := filepath.Join(finalBase, "go")
	if stat, err := os.Stat(filepath.Join(finalRoot, "bin", RunnerBinaryNameForGo(platform.GOOS))); err == nil && !stat.IsDir() {
		return finalBase, nil
	}

	archive, err := m.lookupGoArchive(ctx, version, platform)
	if err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp(filepath.Join(m.layout.GoToolchainsDir, version), "install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, archive.FileName)
	if err := m.downloadWithSHA256(ctx, archive.URL, archive.SHA256, archivePath); err != nil {
		return "", err
	}
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", err
	}
	if err := extractArchive(archivePath, extractDir, platform.GOOS); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(finalBase), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(extractDir, finalBase); err != nil {
		if stat, statErr := os.Stat(finalBase); statErr == nil && stat.IsDir() {
			return finalBase, nil
		}
		return "", err
	}
	return finalBase, nil
}

func (m *Manager) lookupGoArchive(ctx context.Context, version string, platform Platform) (GoArchive, error) {
	archive, err := GoToolchainArchive(version, platform)
	if err != nil {
		return GoArchive{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.indexURL, nil)
	if err != nil {
		return GoArchive{}, err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return GoArchive{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return GoArchive{}, fmt.Errorf("go index request failed with status %d", resp.StatusCode)
	}
	var releases []struct {
		Version string `json:"version"`
		Files   []struct {
			Filename string `json:"filename"`
			OS       string `json:"os"`
			Arch     string `json:"arch"`
			SHA256   string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return GoArchive{}, err
	}
	targetVersion := "go" + normalizeGoVersion(version)
	for _, release := range releases {
		if strings.TrimSpace(release.Version) != targetVersion {
			continue
		}
		for _, file := range release.Files {
			if file.Filename == archive.FileName && file.OS == platform.GOOS && file.Arch == platform.GOARCH {
				archive.SHA256 = strings.TrimSpace(file.SHA256)
				archive.URL = strings.TrimRight(m.downloadBase, "/") + "/" + archive.FileName
				return archive, nil
			}
		}
	}
	return GoArchive{}, fmt.Errorf("go distribution %s for %s/%s not found", version, platform.GOOS, platform.GOARCH)
}

func (m *Manager) downloadWithSHA256(ctx context.Context, sourceURL, wantSHA, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tempFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tempFile.Close()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tempFile, hasher), resp.Body); err != nil {
		return err
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(wantSHA)) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, wantSHA)
	}
	return nil
}

func extractArchive(archivePath, destDir, goos string) error {
	if strings.EqualFold(goos, "windows") {
		return extractZip(archivePath, destDir)
	}
	return extractTarGz(archivePath, destDir)
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target := filepath.Join(destDir, filepath.Clean(file.Name))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			return err
		}
		dst.Close()
		src.Close()
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, filepath.Clean(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(dst, tr); err != nil {
				dst.Close()
				return err
			}
			dst.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}
		}
	}
}

func (m *Manager) buildRunnerBinary(ctx context.Context, repoDir string, repo GitHubRepo, commit string, toolchainBase string) (string, string, error) {
	finalDir := filepath.Join(m.layout.BinDir, repo.Slug, commit)
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return "", "", err
	}
	finalBinary := filepath.Join(finalDir, RunnerBinaryName(runtime.GOOS))
	if stat, err := os.Stat(finalBinary); err == nil && !stat.IsDir() {
		sum, sumErr := fileSHA256(finalBinary)
		return finalBinary, sum, sumErr
	}

	tempDir, err := os.MkdirTemp(finalDir, "build-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tempDir)
	tempBinary := filepath.Join(tempDir, RunnerBinaryName(runtime.GOOS))
	goroot := filepath.Join(toolchainBase, "go")
	env := buildGoBuildEnv(goroot, m.layout)
	cmd := exec.CommandContext(ctx, filepath.Join(goroot, "bin", RunnerBinaryNameForGo(runtime.GOOS)), "build", "-trimpath", "-o", tempBinary, "./cmd/agentcore-runner")
	cmd.Dir = repoDir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("go build failed: %w\n%s", err, output)
	}
	if err := os.Rename(tempBinary, finalBinary); err != nil {
		return "", "", err
	}
	sum, err := fileSHA256(finalBinary)
	if err != nil {
		return "", "", err
	}
	return finalBinary, sum, nil
}

func buildGoBuildEnv(goroot string, layout ManagedLayout) []string {
	pathValue := filepath.Join(goroot, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	envMap := map[string]string{
		"GOROOT":     goroot,
		"GOCACHE":    layout.GoCacheDir,
		"GOMODCACHE": layout.GoModCacheDir,
		"PATH":       pathValue,
	}
	env := os.Environ()
	pairs := make([]string, 0, len(env)+len(envMap))
	seen := make(map[string]struct{}, len(envMap))
	for _, item := range env {
		key := item
		if idx := strings.IndexByte(item, '='); idx >= 0 {
			key = item[:idx]
		}
		if value, ok := envMap[key]; ok {
			pairs = append(pairs, key+"="+value)
			seen[key] = struct{}{}
			continue
		}
		pairs = append(pairs, item)
	}
	for key, value := range envMap {
		if _, ok := seen[key]; ok {
			continue
		}
		pairs = append(pairs, key+"="+value)
	}
	return pairs
}

func RunnerBinaryNameForGo(goos string) string {
	if strings.EqualFold(goos, "windows") {
		return "go.exe"
	}
	return "go"
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func runCommand(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

func finishRunnerExecution(cmd *exec.Cmd, stdin io.Closer, stderr *bytes.Buffer, result RunnerExecutionResult, runErr error) (RunnerExecutionResult, error) {
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd == nil {
		return result, runErr
	}
	waitErr := cmd.Wait()
	result.Stderr = strings.TrimSpace(stderrString(stderr))
	if runErr != nil {
		if waitErr != nil {
			return result, fmt.Errorf("%w; runner wait failed: %v", runErr, waitErr)
		}
		return result, runErr
	}
	if waitErr != nil {
		if result.Stderr != "" {
			return result, fmt.Errorf("runner process failed: %w: %s", waitErr, result.Stderr)
		}
		return result, fmt.Errorf("runner process failed: %w", waitErr)
	}
	return result, nil
}

func stderrString(stderr *bytes.Buffer) string {
	if stderr == nil {
		return ""
	}
	return stderr.String()
}

func runnerTranscriptEntry(direction string, payload map[string]interface{}) RunnerTranscriptEntry {
	entry := RunnerTranscriptEntry{
		Direction: strings.TrimSpace(direction),
		Method:    strings.TrimSpace(fmt.Sprint(payload["method"])),
		ID:        strings.TrimSpace(fmt.Sprint(payload["id"])),
	}
	entry.Text = runnerTranscriptText(payload)
	return entry
}

func runnerTranscriptText(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return ""
	}
	params, _ := payload["params"].(map[string]interface{})
	update, _ := params["update"].(map[string]interface{})
	content, _ := update["content"].(map[string]interface{})
	if text := strings.TrimSpace(fmt.Sprint(content["text"])); text != "" && text != "<nil>" {
		return text
	}
	promptItems, _ := params["prompt"].([]map[string]interface{})
	for _, item := range promptItems {
		if text := strings.TrimSpace(fmt.Sprint(item["text"])); text != "" {
			return text
		}
	}
	promptInterfaces, _ := params["prompt"].([]interface{})
	for _, raw := range promptInterfaces {
		item, _ := raw.(map[string]interface{})
		if text := strings.TrimSpace(fmt.Sprint(item["text"])); text != "" {
			return text
		}
	}
	return ""
}

func jsonRPCErrorMessage(payload map[string]interface{}) string {
	errorPayload, _ := payload["error"].(map[string]interface{})
	if errorPayload == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(errorPayload["message"]))
}

func validatePreparedRunnerBinary(path string, wantSHA string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("runner binary path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("runner binary validation failed: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("runner binary validation failed: %s is a directory", path)
	}
	wantSHA = strings.TrimSpace(wantSHA)
	if wantSHA == "" {
		return nil
	}
	gotSHA, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("runner binary validation failed: %w", err)
	}
	if !strings.EqualFold(gotSHA, wantSHA) {
		return fmt.Errorf("runner binary checksum mismatch: got %s want %s", gotSHA, wantSHA)
	}
	return nil
}

func normalizeGoVersion(version string) string {
	version = strings.TrimSpace(strings.TrimPrefix(version, "go"))
	if version == "" {
		return DefaultGoVersion
	}
	return version
}

func isFullSHA(value string) bool {
	if len(strings.TrimSpace(value)) != 40 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}

func (m *Manager) loadStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(m.layout.StatusPath)
	if err != nil {
		return
	}
	var status Status
	if json.Unmarshal(data, &status) == nil {
		m.status = status
	}
}

func (m *Manager) persistLocked() error {
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return err
	}
	tempPath := m.layout.StatusPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, m.layout.StatusPath)
}

func (m *Manager) failStatus(err error) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastPrepareState = "failed"
	m.status.LastError = strings.TrimSpace(err.Error())
	_ = m.persistLocked()
	return cloneStatus(m.status), err
}

func cloneStatus(status Status) Status {
	out := status
	if status.LastPrepareAt != nil {
		copied := *status.LastPrepareAt
		out.LastPrepareAt = &copied
	}
	if status.LastOptimizationAt != nil {
		copied := *status.LastOptimizationAt
		out.LastOptimizationAt = &copied
	}
	if status.SupportedParts != nil {
		out.SupportedParts = append([]string{}, status.SupportedParts...)
	}
	if status.OptimizedParts != nil {
		out.OptimizedParts = append([]string{}, status.OptimizedParts...)
	}
	return out
}

func enrichStatusWithLastOptimizationSummary(layout ManagedLayout, status *Status) {
	if status == nil {
		return
	}
	runID := strings.TrimSpace(status.LastOptimizationRunID)
	if runID == "" {
		return
	}
	record, err := readOptimizationRunRecord(layout, runID)
	if err != nil {
		return
	}
	status.LastOptimizationAt = parseOptimizationRecordTime(record["created_at"])
	status.LastOptimizationState = summarizeOptimizationState(record)
	status.LastOptimizationSummary = summarizeOptimizationText(record)
	if status.ManifestPath == "" {
		status.ManifestPath = optimizationRecordString(record["manifest_path"])
	}
	if len(status.SupportedParts) == 0 {
		status.SupportedParts = optimizationRecordStrings(record["supported_parts"])
	}
	if len(status.OptimizedParts) == 0 {
		status.OptimizedParts = optimizationRecordStrings(record["optimized_parts"])
	}
	if status.PrimaryPart == "" {
		status.PrimaryPart = optimizationRecordString(record["primary_part"])
	}
	if status.SourceOptimizationRunID == "" {
		status.SourceOptimizationRunID = optimizationRecordString(record["source_optimization_run_id"])
	}
	if status.SourceEvalRunID == "" {
		status.SourceEvalRunID = optimizationRecordString(record["source_eval_run_id"])
	}
}

func readOptimizationRunRecord(layout ManagedLayout, runID string) (OptimizationRunRecord, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	recordPath := filepath.Join(layout.RootDir, "optimization-runs", runID+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		return nil, err
	}
	var record OptimizationRunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return NormalizeOptimizationRunRecordEvolvableParts(record), nil
}

func parseOptimizationRecordTime(value interface{}) *time.Time {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, text)
		if err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

func summarizeOptimizationState(record map[string]interface{}) string {
	if decision := optimizationRecordString(record["followup_decision"]); decision != "" {
		return decision
	}
	if state := optimizationRecordString(record["followup_state"]); state != "" {
		return state
	}
	if runnerError := optimizationRecordString(record["runner_error"]); runnerError != "" {
		return "failed"
	}
	if stopReason := optimizationRecordString(record["runner_stop_reason"]); stopReason != "" {
		return stopReason
	}
	if optimizationRecordString(record["runner_response_text"]) != "" {
		return "completed"
	}
	return ""
}

func summarizeOptimizationText(record map[string]interface{}) string {
	candidates := []string{
		optimizationRecordString(record["followup_summary"]),
		optimizationRecordString(record["followup_message"]),
		optimizationRecordString(record["followup_decision"]),
		optimizationRecordString(record["followup_state"]),
		optimizationRecordString(record["runner_error"]),
		optimizationRecordString(record["runner_response_text"]),
		optimizationRecordString(record["runner_stop_reason"]),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		normalized := strings.Join(strings.Fields(candidate), " ")
		if len(normalized) <= 160 {
			return normalized
		}
		return strings.TrimSpace(normalized[:157]) + "..."
	}
	return ""
}

func optimizationRecordString(value interface{}) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

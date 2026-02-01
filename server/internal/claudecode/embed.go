package claudecode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed bin/*
var embeddedBinaries embed.FS

const (
	// DefaultGCSBucket is the default GCS bucket URL for Claude Code releases.
	DefaultGCSBucket = "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"

	// NPMRegistryURL is the npm registry URL for Claude Code package.
	NPMRegistryURL = "https://registry.npmjs.org/@anthropic-ai/claude-code"

	// DefaultDownloadTimeout is the default timeout for downloading binaries.
	DefaultDownloadTimeout = 5 * time.Minute
)

// BinarySource indicates where the binary came from.
type BinarySource string

const (
	BinarySourceEmbedded     BinarySource = "embedded"
	BinarySourceDownloaded   BinarySource = "downloaded"
	BinarySourceSystem       BinarySource = "system"
	BinarySourceIDEExtension BinarySource = "ide-extension"
)

// VersionInfo contains version information about the Claude Code CLI.
type VersionInfo struct {
	EmbeddedVersion  string       `json:"embedded_version,omitempty"`
	InstalledVersion string       `json:"installed_version,omitempty"`
	SystemVersion    string       `json:"system_version,omitempty"`
	LatestVersion    string       `json:"latest_version,omitempty"`
	ActiveVersion    string       `json:"active_version"`
	Source           BinarySource `json:"source"`
	BinaryPath       string       `json:"binary_path"`
	Platform         string       `json:"platform"`
	UpdateAvailable  bool         `json:"update_available"`
	LastCheck        time.Time    `json:"last_check,omitempty"`
	Validated        bool         `json:"validated"`
}

// Manifest represents the Claude Code release manifest.
type Manifest struct {
	Version   string                     `json:"version"`
	Platforms map[string]PlatformRelease `json:"platforms"`
}

// PlatformRelease contains release info for a specific platform.
type PlatformRelease struct {
	Checksum string `json:"checksum"`
	Size     int64  `json:"size,omitempty"`
}

// BinaryManager handles extraction and management of Claude Code CLI binaries.
type BinaryManager struct {
	extractDir      string
	gcsBaseURL      string
	downloadTimeout time.Duration
	mu              sync.Mutex
	extracted       bool
	httpClient      *http.Client

	// Configuration
	requireLatest  bool // If true, always prefer latest version over system CLI
	allowSystemCLI bool // If true, allow using system-installed CLI

	// Cached version info
	embeddedVersion  string
	installedVersion string
	systemVersion    string
	binaryPath       string
	source           BinarySource
	ideName          string // IDE name if source is BinarySourceIDEExtension
	validated        bool
}

// NewBinaryManager creates a new binary manager.
func NewBinaryManager(extractDir string) *BinaryManager {
	if extractDir == "" {
		// Default to ~/.local/share/zimaos-echo/claude-code
		home, _ := os.UserHomeDir()
		extractDir = filepath.Join(home, ".local", "share", "zimaos-echo", "claude-code")
	}
	return &BinaryManager{
		extractDir:      extractDir,
		gcsBaseURL:      DefaultGCSBucket,
		downloadTimeout: DefaultDownloadTimeout,
		allowSystemCLI:  true, // Allow system CLI by default
		requireLatest:   false, // Don't require latest by default
		httpClient: &http.Client{
			Timeout: DefaultDownloadTimeout,
		},
	}
}

// SetRequireLatest sets whether to require the latest version.
// If true, system CLI will only be used if it's the latest version.
func (m *BinaryManager) SetRequireLatest(require bool) {
	m.requireLatest = require
}

// SetAllowSystemCLI sets whether to allow using system-installed CLI.
func (m *BinaryManager) SetAllowSystemCLI(allow bool) {
	m.allowSystemCLI = allow
}

// SetGCSBaseURL sets a custom GCS base URL for downloads.
func (m *BinaryManager) SetGCSBaseURL(url string) {
	m.gcsBaseURL = url
}

// SetDownloadTimeout sets the download timeout.
func (m *BinaryManager) SetDownloadTimeout(timeout time.Duration) {
	m.downloadTimeout = timeout
	m.httpClient.Timeout = timeout
}

// GetBinaryPath returns the path to the Claude Code CLI binary for the current platform.
// Resolution order:
// 1. Embedded binary (if available for current platform)
// 2. Downloaded binary (if exists)
// 3. IDE extension binary (VS Code, Cursor, etc.)
// 4. System PATH (if allowed and validated)
// 5. Download from GCS (if none found)
//
// If requireLatest is true, system CLI will only be used if it matches the latest version.
func (m *BinaryManager) GetBinaryPath() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	platform := getPlatformString()
	binaryName := "claude"
	if runtime.GOOS == "windows" {
		binaryName = "claude.exe"
	}

	binaryPath := filepath.Join(m.extractDir, binaryName)

	// Check if already resolved
	if m.extracted && m.binaryPath != "" {
		if _, err := os.Stat(m.binaryPath); err == nil {
			return m.binaryPath, nil
		}
	}

	// Try 1: Use embedded binary if available
	if m.isEmbeddedForPlatform(platform) {
		if err := m.extractBinary(platform, binaryPath); err == nil {
			m.extracted = true
			m.binaryPath = binaryPath
			m.source = BinarySourceEmbedded
			m.embeddedVersion, _ = m.getEmbeddedVersion()
			m.installedVersion = m.embeddedVersion
			m.validated = true
			return binaryPath, nil
		}
	}

	// Try 2: Check if already downloaded
	if _, err := os.Stat(binaryPath); err == nil {
		m.extracted = true
		m.binaryPath = binaryPath
		m.source = BinarySourceDownloaded
		m.installedVersion, _ = m.getInstalledVersionFromFile()
		m.validated = true
		return binaryPath, nil
	}

	// Try 3: Check IDE extension paths (VS Code, Cursor, etc.)
	if idePath, ideName := m.findCLIInIDEExtensions(platform, binaryName); idePath != "" {
		m.extracted = true
		m.binaryPath = idePath
		m.source = BinarySourceIDEExtension
		m.ideName = ideName
		// Try to get version from the binary
		if version, err := m.getVersionFromBinary(idePath); err == nil {
			m.installedVersion = version
		}
		m.validated = true
		return idePath, nil
	}

	// Try 4: Check system PATH (if allowed)
	if m.allowSystemCLI {
		if systemPath, version, err := m.findAndValidateSystemCLI(binaryName); err == nil {
			// Check if we require latest version
			if m.requireLatest {
				latestVersion, err := m.fetchLatestVersion()
				if err == nil && version != latestVersion {
					// System CLI is not latest, skip to download
					goto download
				}
			}
			m.extracted = true
			m.binaryPath = systemPath
			m.source = BinarySourceSystem
			m.systemVersion = version
			m.installedVersion = version
			m.validated = true
			return systemPath, nil
		}
	}

download:
	// Try 5: Download from GCS
	if err := m.downloadLatest(platform, binaryPath); err != nil {
		return "", fmt.Errorf("claude code CLI not found and download failed: %w", err)
	}

	m.extracted = true
	m.binaryPath = binaryPath
	m.source = BinarySourceDownloaded
	m.validated = true
	return binaryPath, nil
}

// isEmbeddedForPlatform checks if a binary is embedded for the given platform.
func (m *BinaryManager) isEmbeddedForPlatform(platform string) bool {
	srcName := fmt.Sprintf("claude-%s", platform)
	if runtime.GOOS == "windows" {
		srcName += ".exe"
	}
	srcPath := filepath.Join("bin", srcName)

	_, err := embeddedBinaries.ReadFile(srcPath)
	return err == nil
}

// extractBinary extracts the embedded binary for the given platform.
func (m *BinaryManager) extractBinary(platform, destPath string) error {
	// Determine source filename
	srcName := fmt.Sprintf("claude-%s", platform)
	if runtime.GOOS == "windows" {
		srcName += ".exe"
	}
	srcPath := filepath.Join("bin", srcName)

	// Read embedded binary
	data, err := embeddedBinaries.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("embedded binary not found for platform %s: %w", platform, err)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write binary
	if err := os.WriteFile(destPath, data, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	// Also write version file if embedded
	if version, err := m.getEmbeddedVersion(); err == nil {
		versionPath := filepath.Join(filepath.Dir(destPath), "VERSION")
		os.WriteFile(versionPath, []byte(version), 0644)
	}

	return nil
}

// downloadLatest downloads the latest Claude Code CLI binary.
// It tries GCS first, and uses npm registry as fallback for version info only.
func (m *BinaryManager) downloadLatest(platform, destPath string) error {
	// Try to get latest version from GCS first
	version, err := m.fetchLatestVersion()
	if err != nil {
		// Fallback to npm for version info only
		version, err = m.fetchLatestVersionFromNPM()
		if err != nil {
			return fmt.Errorf("failed to get latest version from both GCS and npm: %w", err)
		}
	}

	// Download from GCS (the only source for standalone binaries)
	if err := m.downloadVersion(version, platform, destPath); err != nil {
		return fmt.Errorf("failed to download from GCS: %w", err)
	}

	return nil
}

// downloadVersion downloads a specific version of Claude Code CLI.
func (m *BinaryManager) downloadVersion(version, platform, destPath string) error {
	// Get manifest for checksum
	manifest, err := m.fetchManifest(version)
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	platformInfo, ok := manifest.Platforms[platform]
	if !ok {
		return fmt.Errorf("platform %s not found in manifest", platform)
	}

	// Build download URL
	binaryName := "claude"
	if platform == "win32-x64" {
		binaryName = "claude.exe"
	}
	downloadURL := fmt.Sprintf("%s/%s/%s/%s", m.gcsBaseURL, version, platform, binaryName)

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download binary
	resp, err := m.httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temp file
	tmpPath := destPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Download and calculate checksum
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	_, err = io.Copy(writer, resp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Verify checksum
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != platformInfo.Checksum {
		os.Remove(tmpPath)
		return fmt.Errorf("checksum mismatch: expected %s, got %s", platformInfo.Checksum, actualChecksum)
	}

	// Move to final location
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move binary: %w", err)
	}

	// Write version file
	versionPath := filepath.Join(filepath.Dir(destPath), "VERSION")
	os.WriteFile(versionPath, []byte(version), 0644)

	m.installedVersion = version
	return nil
}

// fetchLatestVersion fetches the latest version string from GCS.
func (m *BinaryManager) fetchLatestVersion() (string, error) {
	url := fmt.Sprintf("%s/latest", m.gcsBaseURL)
	resp, err := m.httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest version: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// fetchLatestVersionFromNPM fetches the latest version from npm registry.
// This is used as a fallback when GCS version check fails.
func (m *BinaryManager) fetchLatestVersionFromNPM() (string, error) {
	resp, err := m.httpClient.Get(NPMRegistryURL + "/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch npm package info: status %d", resp.StatusCode)
	}

	var pkgInfo struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkgInfo); err != nil {
		return "", err
	}

	return pkgInfo.Version, nil
}

// fetchManifest fetches the manifest for a specific version.
func (m *BinaryManager) fetchManifest(version string) (*Manifest, error) {
	url := fmt.Sprintf("%s/%s/manifest.json", m.gcsBaseURL, version)
	resp, err := m.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: status %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// findInSystemPath searches for the binary in system PATH.
func (m *BinaryManager) findInSystemPath(binaryName string) (string, error) {
	// Check common locations first
	locations := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "bin", binaryName),
		filepath.Join("/usr", "local", "bin", binaryName),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc, nil
		}
	}

	// Try PATH
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH environment variable is empty")
	}

	pathSep := string(os.PathListSeparator)
	for _, dir := range strings.Split(pathEnv, pathSep) {
		path := filepath.Join(dir, binaryName)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("executable %s not found in PATH", binaryName)
}

// findAndValidateSystemCLI finds the CLI in system PATH and validates it's a real Claude Code CLI.
// Returns the path, version, and any error.
func (m *BinaryManager) findAndValidateSystemCLI(binaryName string) (string, string, error) {
	// Find the binary first
	path, err := m.findInSystemPath(binaryName)
	if err != nil {
		return "", "", err
	}

	// Validate it's a real Claude Code CLI by running --version
	version, err := m.getVersionFromBinary(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to validate CLI at %s: %w", path, err)
	}

	// Optionally do a dry run to verify it works
	if err := m.dryRunCLI(path); err != nil {
		return "", "", fmt.Errorf("CLI dry run failed at %s: %w", path, err)
	}

	return path, version, nil
}

// getVersionFromBinary executes the CLI with --version flag and parses the output.
func (m *BinaryManager) getVersionFromBinary(binaryPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute --version: %w", err)
	}

	// Parse version from output
	// Expected format: "claude <version>" or similar
	version := m.parseVersionOutput(string(output))
	if version == "" {
		return "", fmt.Errorf("could not parse version from output: %s", string(output))
	}

	return version, nil
}

// parseVersionOutput extracts version string from CLI output.
func (m *BinaryManager) parseVersionOutput(output string) string {
	output = strings.TrimSpace(output)

	// Try to match common version patterns
	// Pattern 1: "claude 1.0.0" or "Claude Code 1.0.0"
	// Pattern 2: "1.0.0"
	// Pattern 3: "v1.0.0"

	// Regex to match semantic version
	versionRegex := regexp.MustCompile(`v?(\d+\.\d+\.\d+(?:-[a-zA-Z0-9.]+)?)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}

	// If output is just a version number
	if regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(output) {
		return output
	}

	return ""
}

// dryRunCLI performs a dry run to verify the CLI is functional.
// This runs a minimal command that doesn't require API keys or network.
func (m *BinaryManager) dryRunCLI(binaryPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try --help first as it's the safest option
	cmd := exec.CommandContext(ctx, binaryPath, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check if it's just a non-zero exit code but still produced help output
		if stdout.Len() > 0 && strings.Contains(strings.ToLower(stdout.String()), "claude") {
			return nil
		}
		return fmt.Errorf("dry run failed: %w, stderr: %s", err, stderr.String())
	}

	// Verify the output looks like Claude Code CLI help
	output := stdout.String()
	if !strings.Contains(strings.ToLower(output), "claude") {
		return fmt.Errorf("output does not appear to be from Claude Code CLI")
	}

	return nil
}

// GetSystemCLIVersion returns the version of the system-installed CLI if available.
func (m *BinaryManager) GetSystemCLIVersion() (string, error) {
	if m.systemVersion != "" {
		return m.systemVersion, nil
	}

	binaryName := "claude"
	if runtime.GOOS == "windows" {
		binaryName = "claude.exe"
	}

	path, err := m.findInSystemPath(binaryName)
	if err != nil {
		return "", err
	}

	version, err := m.getVersionFromBinary(path)
	if err != nil {
		return "", err
	}

	m.systemVersion = version
	return version, nil
}

// ValidateBinary validates a CLI binary at the given path.
// Returns the version and any validation error.
func (m *BinaryManager) ValidateBinary(binaryPath string) (string, error) {
	version, err := m.getVersionFromBinary(binaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	if err := m.dryRunCLI(binaryPath); err != nil {
		return version, fmt.Errorf("dry run failed: %w", err)
	}

	return version, nil
}

// DryRun performs a dry run validation on the current binary.
func (m *BinaryManager) DryRun() error {
	if m.binaryPath == "" {
		return fmt.Errorf("no binary path set")
	}
	return m.dryRunCLI(m.binaryPath)
}

// getEmbeddedVersion returns the version of the embedded Claude Code CLI.
func (m *BinaryManager) getEmbeddedVersion() (string, error) {
	data, err := embeddedBinaries.ReadFile("bin/VERSION")
	if err != nil {
		return "", fmt.Errorf("version file not found: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// getInstalledVersionFromFile reads the version from the installed VERSION file.
func (m *BinaryManager) getInstalledVersionFromFile() (string, error) {
	versionPath := filepath.Join(m.extractDir, "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// GetVersion returns the version of the embedded Claude Code CLI.
func (m *BinaryManager) GetVersion() (string, error) {
	return m.getEmbeddedVersion()
}

// GetInstalledVersion returns the version of the installed Claude Code CLI.
func (m *BinaryManager) GetInstalledVersion() (string, error) {
	if m.installedVersion != "" {
		return m.installedVersion, nil
	}
	return m.getInstalledVersionFromFile()
}

// IsEmbedded returns true if Claude Code CLI binaries are embedded.
func (m *BinaryManager) IsEmbedded() bool {
	entries, err := embeddedBinaries.ReadDir("bin")
	if err != nil {
		return false
	}
	// Check if there are actual binary files (not just VERSION or .gitkeep)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "claude-") {
			return true
		}
	}
	return false
}

// IsEmbeddedForCurrentPlatform returns true if a binary is embedded for the current platform.
func (m *BinaryManager) IsEmbeddedForCurrentPlatform() bool {
	return m.isEmbeddedForPlatform(getPlatformString())
}

// GetSource returns the source of the current binary.
func (m *BinaryManager) GetSource() BinarySource {
	return m.source
}

// GetVersionInfo returns comprehensive version information.
// This method returns partial information even if no binary is available,
// allowing the UI to show status and offer download/update options.
func (m *BinaryManager) GetVersionInfo() (*VersionInfo, error) {
	info := &VersionInfo{
		Platform:  getPlatformString(),
		Source:    m.source,
		Validated: m.validated,
	}

	// Get embedded version if available
	if embVer, err := m.getEmbeddedVersion(); err == nil {
		info.EmbeddedVersion = embVer
	}

	// Get system version if available (don't fail if not found)
	if sysVer, err := m.GetSystemCLIVersion(); err == nil {
		info.SystemVersion = sysVer
	}

	// Try to get binary path - this may trigger download
	binaryPath, err := m.GetBinaryPath()
	if err != nil {
		// Binary not available - return partial info with error context
		// This allows UI to show "not installed" state and offer download
		info.BinaryPath = ""
		info.ActiveVersion = ""
		info.Validated = false
		// Return the info without error so UI can display it
		return info, nil
	}

	info.BinaryPath = binaryPath

	// Get installed version
	if instVer, err := m.GetInstalledVersion(); err == nil {
		info.InstalledVersion = instVer
		info.ActiveVersion = instVer
	}

	return info, nil
}

// CheckForUpdates checks if a newer version is available.
func (m *BinaryManager) CheckForUpdates() (bool, string, error) {
	latestVersion, err := m.fetchLatestVersion()
	if err != nil {
		return false, "", err
	}

	currentVersion, err := m.GetInstalledVersion()
	if err != nil {
		// No installed version, update is available
		return true, latestVersion, nil
	}

	// Simple string comparison (versions are semver-like)
	updateAvailable := latestVersion != currentVersion
	return updateAvailable, latestVersion, nil
}

// Update updates to the specified version (or latest if empty).
func (m *BinaryManager) Update(version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	platform := getPlatformString()
	binaryName := "claude"
	if runtime.GOOS == "windows" {
		binaryName = "claude.exe"
	}
	binaryPath := filepath.Join(m.extractDir, binaryName)

	// Get version to download
	if version == "" || version == "latest" {
		var err error
		version, err = m.fetchLatestVersion()
		if err != nil {
			return fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	// Backup current binary
	backupPath := binaryPath + ".backup"
	if _, err := os.Stat(binaryPath); err == nil {
		os.Rename(binaryPath, backupPath)
	}

	// Download new version
	if err := m.downloadVersion(version, platform, binaryPath); err != nil {
		// Restore backup on failure
		if _, err := os.Stat(backupPath); err == nil {
			os.Rename(backupPath, binaryPath)
		}
		return err
	}

	// Remove backup
	os.Remove(backupPath)

	m.binaryPath = binaryPath
	m.source = BinarySourceDownloaded
	m.installedVersion = version
	return nil
}

// ListEmbeddedPlatforms returns a list of embedded platform binaries.
func (m *BinaryManager) ListEmbeddedPlatforms() []string {
	entries, err := embeddedBinaries.ReadDir("bin")
	if err != nil {
		return nil
	}

	var platforms []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "claude-") {
			// Extract platform from filename (e.g., "claude-darwin-arm64" -> "darwin-arm64")
			platform := strings.TrimPrefix(name, "claude-")
			platform = strings.TrimSuffix(platform, ".exe")
			platforms = append(platforms, platform)
		}
	}
	return platforms
}

// ExtractAll extracts all embedded binaries to the specified directory.
func (m *BinaryManager) ExtractAll(destDir string) error {
	entries, err := embeddedBinaries.ReadDir("bin")
	if err != nil {
		return fmt.Errorf("failed to read embedded binaries: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join("bin", entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		srcFile, err := embeddedBinaries.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", entry.Name(), err)
		}

		destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to create %s: %w", destPath, err)
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			return fmt.Errorf("failed to copy %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// ClearCache removes downloaded binaries from the cache directory.
func (m *BinaryManager) ClearCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove all files in extract directory
	entries, err := os.ReadDir(m.extractDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(m.extractDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	m.extracted = false
	m.binaryPath = ""
	m.installedVersion = ""
	return nil
}

// getPlatformString returns the platform string for the current OS/arch.
func getPlatformString() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go's GOOS/GOARCH to Claude Code's platform strings
	switch os {
	case "darwin":
		switch arch {
		case "amd64":
			return "darwin-x64"
		case "arm64":
			return "darwin-arm64"
		}
	case "linux":
		switch arch {
		case "amd64":
			return "linux-x64"
		case "arm64":
			return "linux-arm64"
		}
	case "windows":
		switch arch {
		case "amd64":
			return "win32-x64"
		}
	}

	return fmt.Sprintf("%s-%s", os, arch)
}

// bmIDEExtensionInfo contains information about an IDE extension location for BinaryManager.
type bmIDEExtensionInfo struct {
	name       string   // IDE name (e.g., "VS Code", "Cursor")
	baseDirs   []string // Base directories to search (relative to home or absolute)
	extPattern string   // Extension directory pattern (glob)
}

// findCLIInIDEExtensions searches for Claude Code CLI in common IDE extension directories.
func (m *BinaryManager) findCLIInIDEExtensions(platform, binaryName string) (path string, ideName string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}

	// Build the relative path to the binary within the extension
	// Format: resources/native-binary/<platform>/claude[.exe]
	binaryRelPath := filepath.Join("resources", "native-binary", platform, binaryName)

	// Define IDE extension locations
	ideInfos := []bmIDEExtensionInfo{
		// Cursor (VS Code fork) - most common for Claude Code
		{
			name:       "Cursor",
			baseDirs:   []string{".cursor/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// VS Code
		{
			name:       "VS Code",
			baseDirs:   []string{".vscode/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// VS Code Insiders
		{
			name:       "VS Code Insiders",
			baseDirs:   []string{".vscode-insiders/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// VSCodium
		{
			name:       "VSCodium",
			baseDirs:   []string{".vscode-oss/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Windsurf (Codeium's VS Code fork)
		{
			name:       "Windsurf",
			baseDirs:   []string{".windsurf/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Positron (Posit's VS Code fork for data science)
		{
			name:       "Positron",
			baseDirs:   []string{".positron/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Theia IDE
		{
			name:       "Theia",
			baseDirs:   []string{".theia/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Qodo (formerly Codium AI)
		{
			name:       "Qodo",
			baseDirs:   []string{".qodo/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// TRAE (ByteDance's AI IDE)
		{
			name:       "TRAE",
			baseDirs:   []string{".trae/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Kiro (Amazon's AI IDE)
		{
			name:       "Kiro",
			baseDirs:   []string{".kiro/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Zed Editor
		{
			name:       "Zed",
			baseDirs:   []string{".zed/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Fleet (JetBrains)
		{
			name:       "Fleet",
			baseDirs:   []string{".fleet/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Void (Open source AI IDE)
		{
			name:       "Void",
			baseDirs:   []string{".void/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Melty (AI-native IDE)
		{
			name:       "Melty",
			baseDirs:   []string{".melty/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// PearAI
		{
			name:       "PearAI",
			baseDirs:   []string{".pearai/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Qoder (Chinese AI IDE)
		{
			name:       "Qoder",
			baseDirs:   []string{".qoder/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// MarsCode (ByteDance's Chinese AI IDE)
		{
			name:       "MarsCode",
			baseDirs:   []string{".marscode/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Aide (Open source AI IDE)
		{
			name:       "Aide",
			baseDirs:   []string{".aide/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Augment Code
		{
			name:       "Augment",
			baseDirs:   []string{".augment/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Antigravity (AI IDE with Gemini/Claude integration)
		{
			name:       "Antigravity",
			baseDirs:   []string{".antigravity/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Roo Code (VS Code fork)
		{
			name:       "Roo Code",
			baseDirs:   []string{".roo-code/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
		// Kiro (AI code assistant)
		{
			name:       "Kiro",
			baseDirs:   []string{".kiro/extensions"},
			extPattern: "anthropic.claude-code-*",
		},
	}

	// Add platform-specific paths
	if runtime.GOOS == "windows" {
		// Windows: also check AppData locations
		appData := os.Getenv("APPDATA")
		localAppData := os.Getenv("LOCALAPPDATA")

		if appData != "" {
			ideInfos = append(ideInfos,
				bmIDEExtensionInfo{
					name:       "VS Code (AppData)",
					baseDirs:   []string{filepath.Join(appData, "Code", "User", "globalStorage")},
					extPattern: "anthropic.claude-code-*",
				},
			)
		}
		if localAppData != "" {
			ideInfos = append(ideInfos,
				bmIDEExtensionInfo{
					name:       "Cursor (LocalAppData)",
					baseDirs:   []string{filepath.Join(localAppData, "Programs", "cursor", "resources", "app", "extensions")},
					extPattern: "anthropic.claude-code-*",
				},
			)
		}
	} else if runtime.GOOS == "darwin" {
		// macOS: check Application Support
		ideInfos = append(ideInfos,
			bmIDEExtensionInfo{
				name:       "VS Code (macOS)",
				baseDirs:   []string{"Library/Application Support/Code/User/globalStorage"},
				extPattern: "anthropic.claude-code-*",
			},
			bmIDEExtensionInfo{
				name:       "Cursor (macOS)",
				baseDirs:   []string{"Library/Application Support/Cursor/User/globalStorage"},
				extPattern: "anthropic.claude-code-*",
			},
		)
	} else if runtime.GOOS == "linux" {
		// Linux: check .config locations
		ideInfos = append(ideInfos,
			bmIDEExtensionInfo{
				name:       "VS Code (Linux)",
				baseDirs:   []string{".config/Code/User/globalStorage"},
				extPattern: "anthropic.claude-code-*",
			},
			bmIDEExtensionInfo{
				name:       "Cursor (Linux)",
				baseDirs:   []string{".config/Cursor/User/globalStorage"},
				extPattern: "anthropic.claude-code-*",
			},
		)
	}

	// Search each IDE location
	for _, info := range ideInfos {
		for _, baseDir := range info.baseDirs {
			var searchDir string
			if filepath.IsAbs(baseDir) {
				searchDir = baseDir
			} else {
				searchDir = filepath.Join(home, baseDir)
			}

			// Find extension directories matching pattern
			pattern := filepath.Join(searchDir, info.extPattern)
			matches, err := filepath.Glob(pattern)
			if err != nil || len(matches) == 0 {
				continue
			}

			// Sort matches to get the latest version (assuming version is in directory name)
			// The glob pattern will match directories like "anthropic.claude-code-2.1.27-win32-x64"
			// We want the highest version number, so iterate from the end
			for i := len(matches) - 1; i >= 0; i-- {
				extDir := matches[i]
				fullBinaryPath := filepath.Join(extDir, binaryRelPath)

				// Check if binary exists and is executable
				if stat, err := os.Stat(fullBinaryPath); err == nil && !stat.IsDir() {
					return fullBinaryPath, info.name
				}
			}
		}
	}

	return "", ""
}

// GetIDEName returns the IDE name if the binary was found in an IDE extension.
func (m *BinaryManager) GetIDEName() string {
	return m.ideName
}

// DefaultBinaryManager is the default binary manager instance.
var DefaultBinaryManager = NewBinaryManager("")

// GetClaudeCodePath returns the path to the Claude Code CLI binary.
// It uses the embedded binary if available, otherwise downloads or falls back to system PATH.
func GetClaudeCodePath() (string, error) {
	return DefaultBinaryManager.GetBinaryPath()
}

// LookPath searches for an executable in the PATH.
func LookPath(file string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH environment variable is empty")
	}

	pathSep := string(os.PathListSeparator)
	for _, dir := range strings.Split(pathEnv, pathSep) {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("executable %s not found in PATH", file)
}

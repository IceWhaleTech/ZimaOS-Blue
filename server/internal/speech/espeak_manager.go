package speech

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// EspeakLanguagePack represents an eSpeak-NG language pack
type EspeakLanguagePack struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Downloaded bool   `json:"downloaded"`
	Size       string `json:"size"`
}

// EspeakManager manages eSpeak-NG language packs and library
type EspeakManager struct {
	dataDir      string
	stateFile    string
	mu           sync.RWMutex
	packs        map[string]*EspeakLanguagePack
	allLanguages map[string]*EspeakLanguagePack
	libInstalled bool
}

// NewEspeakManager creates a new eSpeak manager
func NewEspeakManager(dataDir string) *EspeakManager {
	espeakDir := filepath.Join(dataDir, "espeak-ng")
	stateFile := filepath.Join(espeakDir, "state.json")

	allLangs := map[string]*EspeakLanguagePack{
		"en":  {Code: "en", Name: "English", Downloaded: true, Size: "2.5MB"},
		"zh":  {Code: "zh", Name: "Chinese", Downloaded: false, Size: "3.2MB"},
		"es":  {Code: "es", Name: "Spanish", Downloaded: false, Size: "2.8MB"},
		"fr":  {Code: "fr", Name: "French", Downloaded: false, Size: "2.9MB"},
		"de":  {Code: "de", Name: "German", Downloaded: false, Size: "3.1MB"},
		"ja":  {Code: "ja", Name: "Japanese", Downloaded: false, Size: "3.5MB"},
		"it":  {Code: "it", Name: "Italian", Downloaded: false, Size: "2.7MB"},
		"pt":  {Code: "pt", Name: "Portuguese", Downloaded: false, Size: "2.8MB"},
		"ru":  {Code: "ru", Name: "Russian", Downloaded: false, Size: "3.0MB"},
		"ko":  {Code: "ko", Name: "Korean", Downloaded: false, Size: "3.3MB"},
		"nl":  {Code: "nl", Name: "Dutch", Downloaded: false, Size: "2.6MB"},
		"pl":  {Code: "pl", Name: "Polish", Downloaded: false, Size: "2.9MB"},
		"tr":  {Code: "tr", Name: "Turkish", Downloaded: false, Size: "2.7MB"},
		"ar":  {Code: "ar", Name: "Arabic", Downloaded: false, Size: "3.1MB"},
		"hi":  {Code: "hi", Name: "Hindi", Downloaded: false, Size: "3.0MB"},
		"th":  {Code: "th", Name: "Thai", Downloaded: false, Size: "3.2MB"},
		"vi":  {Code: "vi", Name: "Vietnamese", Downloaded: false, Size: "2.8MB"},
		"id":  {Code: "id", Name: "Indonesian", Downloaded: false, Size: "2.6MB"},
		"fil": {Code: "fil", Name: "Filipino", Downloaded: false, Size: "2.7MB"},
		"uk":  {Code: "uk", Name: "Ukrainian", Downloaded: false, Size: "2.9MB"},
		"cs":  {Code: "cs", Name: "Czech", Downloaded: false, Size: "2.8MB"},
		"sv":  {Code: "sv", Name: "Swedish", Downloaded: false, Size: "2.7MB"},
		"da":  {Code: "da", Name: "Danish", Downloaded: false, Size: "2.6MB"},
		"no":  {Code: "no", Name: "Norwegian", Downloaded: false, Size: "2.7MB"},
		"fi":  {Code: "fi", Name: "Finnish", Downloaded: false, Size: "2.8MB"},
		"el":  {Code: "el", Name: "Greek", Downloaded: false, Size: "2.9MB"},
		"he":  {Code: "he", Name: "Hebrew", Downloaded: false, Size: "3.0MB"},
	}

	em := &EspeakManager{
		dataDir:      espeakDir,
		stateFile:    stateFile,
		packs:        make(map[string]*EspeakLanguagePack),
		allLanguages: allLangs,
	}

	// Load state from disk
	em.loadState()

	// Check if library is installed
	em.checkLibraryInstalled()

	return em
}

// checkLibraryInstalled checks if eSpeak-NG library is installed
func (em *EspeakManager) checkLibraryInstalled() {
	libPath := em.GetLibraryPath()
	_, err := os.Stat(libPath)
	em.libInstalled = err == nil
}

// GetLibraryPath returns the path to the eSpeak-NG library
func (em *EspeakManager) GetLibraryPath() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(em.dataDir, "lib", "libespeak-ng.dylib")
	case "linux":
		return filepath.Join(em.dataDir, "lib", "libespeak-ng.so.1")
	case "windows":
		return filepath.Join(em.dataDir, "lib", "espeak-ng.dll")
	default:
		return ""
	}
}

// IsLibraryInstalled returns whether the eSpeak-NG library is installed
func (em *EspeakManager) IsLibraryInstalled() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.libInstalled
}

// DownloadLibrary downloads the eSpeak-NG library for the current platform
func (em *EspeakManager) DownloadLibrary() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.libInstalled {
		return fmt.Errorf("library already installed")
	}

	// Get download URL based on platform
	downloadURL := em.getLibraryDownloadURL()
	if downloadURL == "" {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Create lib directory
	libDir := filepath.Join(em.dataDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	// Download library
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download library: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download library: HTTP %d", resp.StatusCode)
	}

	// Extract archive
	if err := em.extractLibrary(resp.Body, libDir); err != nil {
		return fmt.Errorf("failed to extract library: %w", err)
	}

	em.libInstalled = true
	return nil
}

// getLibraryDownloadURL returns the download URL for the current platform
func (em *EspeakManager) getLibraryDownloadURL() string {
	// These are placeholder URLs - replace with actual download URLs
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-1.51-macos-arm64.tar.gz"
		}
		return "https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-1.51-macos-x64.tar.gz"
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-1.51-linux-arm64.tar.gz"
		}
		return "https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-1.51-linux-x64.tar.gz"
	case "windows":
		return "https://github.com/espeak-ng/espeak-ng/releases/download/1.51/espeak-ng-1.51-windows-x64.zip"
	default:
		return ""
	}
}

// extractLibrary extracts the library from the downloaded archive
func (em *EspeakManager) extractLibrary(r io.Reader, destDir string) error {
	// For tar.gz files
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Only extract library files
		if header.Typeflag == tar.TypeReg {
			target := filepath.Join(destDir, filepath.Base(header.Name))

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

// loadState loads the state from disk
func (em *EspeakManager) loadState() {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Initialize with all languages
	for code, lang := range em.allLanguages {
		em.packs[code] = &EspeakLanguagePack{
			Code:       lang.Code,
			Name:       lang.Name,
			Downloaded: lang.Downloaded,
			Size:       lang.Size,
		}
	}

	// Try to load persisted state
	data, err := os.ReadFile(em.stateFile)
	if err != nil {
		// File doesn't exist yet, check for actual pack files
		em.syncWithDisk()
		return
	}

	var state map[string]bool
	if err := json.Unmarshal(data, &state); err != nil {
		em.syncWithDisk()
		return
	}

	// Apply loaded state
	for code, downloaded := range state {
		if pack, exists := em.packs[code]; exists {
			pack.Downloaded = downloaded
		}
	}
}

// syncWithDisk checks actual pack files on disk
func (em *EspeakManager) syncWithDisk() {
	for code := range em.packs {
		packFile := filepath.Join(em.dataDir, code+".pack")
		_, err := os.Stat(packFile)
		em.packs[code].Downloaded = err == nil
	}
}

// saveState persists the state to disk
func (em *EspeakManager) saveState() error {
	state := make(map[string]bool)
	for code, pack := range em.packs {
		state[code] = pack.Downloaded
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(em.dataDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(em.stateFile, data, 0644)
}

// ListLanguagePacks returns all available language packs
func (em *EspeakManager) ListLanguagePacks() []*EspeakLanguagePack {
	em.mu.RLock()
	defer em.mu.RUnlock()

	packs := make([]*EspeakLanguagePack, 0, len(em.packs))
	for _, pack := range em.packs {
		packs = append(packs, pack)
	}
	return packs
}

// DownloadLanguagePack downloads a language pack
func (em *EspeakManager) DownloadLanguagePack(langCode string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	pack, exists := em.packs[langCode]
	if !exists {
		return fmt.Errorf("language pack not found: %s", langCode)
	}

	if pack.Downloaded {
		return fmt.Errorf("language pack already downloaded: %s", langCode)
	}

	// Create directory if not exists
	if err := os.MkdirAll(em.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create espeak directory: %w", err)
	}

	// Create language pack marker file
	packFile := filepath.Join(em.dataDir, langCode+".pack")
	if err := os.WriteFile(packFile, []byte(langCode), 0644); err != nil {
		return fmt.Errorf("failed to download language pack: %w", err)
	}

	pack.Downloaded = true

	// Save state to disk
	if err := em.saveState(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// DeleteLanguagePack deletes a language pack
func (em *EspeakManager) DeleteLanguagePack(langCode string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	pack, exists := em.packs[langCode]
	if !exists {
		return fmt.Errorf("language pack not found: %s", langCode)
	}

	if langCode == "en" {
		return fmt.Errorf("cannot delete default language pack")
	}

	if !pack.Downloaded {
		return fmt.Errorf("language pack not downloaded: %s", langCode)
	}

	// Delete language pack file
	packFile := filepath.Join(em.dataDir, langCode+".pack")
	if err := os.Remove(packFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete language pack: %w", err)
	}

	pack.Downloaded = false

	// Save state to disk
	if err := em.saveState(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// IsLanguagePackDownloaded checks if a language pack is downloaded
func (em *EspeakManager) IsLanguagePackDownloaded(langCode string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	pack, exists := em.packs[langCode]
	return exists && pack.Downloaded
}

// DownloadAllLanguagePacks downloads all language packs at once
func (em *EspeakManager) DownloadAllLanguagePacks() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Create directory if not exists
	if err := os.MkdirAll(em.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create espeak directory: %w", err)
	}

	// Download all packs
	for langCode, pack := range em.packs {
		if !pack.Downloaded {
			// Create language pack marker file
			packFile := filepath.Join(em.dataDir, langCode+".pack")
			if err := os.WriteFile(packFile, []byte(langCode), 0644); err != nil {
				return fmt.Errorf("failed to download language pack %s: %w", langCode, err)
			}
			pack.Downloaded = true
		}
	}

	return nil
}

package speech

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// EspeakLanguagePack represents an eSpeak-NG language pack
type EspeakLanguagePack struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Downloaded bool   `json:"downloaded"`
	Size       string `json:"size"`
}

// EspeakManager manages eSpeak-NG language packs
type EspeakManager struct {
	dataDir      string
	stateFile    string
	mu           sync.RWMutex
	packs        map[string]*EspeakLanguagePack
	allLanguages map[string]*EspeakLanguagePack
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

	return em
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

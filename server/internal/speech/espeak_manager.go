package speech

import (
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
	dataDir string
	mu      sync.RWMutex
	packs   map[string]*EspeakLanguagePack
}

// NewEspeakManager creates a new eSpeak manager
func NewEspeakManager(dataDir string) *EspeakManager {
	return &EspeakManager{
		dataDir: filepath.Join(dataDir, "espeak-ng"),
		packs: map[string]*EspeakLanguagePack{
			"en": {Code: "en", Name: "English", Downloaded: true, Size: "2.5MB"},
			"zh": {Code: "zh", Name: "Chinese", Downloaded: false, Size: "3.2MB"},
			"es": {Code: "es", Name: "Spanish", Downloaded: false, Size: "2.8MB"},
			"fr": {Code: "fr", Name: "French", Downloaded: false, Size: "2.9MB"},
			"de": {Code: "de", Name: "German", Downloaded: false, Size: "3.1MB"},
			"ja": {Code: "ja", Name: "Japanese", Downloaded: false, Size: "3.5MB"},
		},
	}
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
	return nil
}

// IsLanguagePackDownloaded checks if a language pack is downloaded
func (em *EspeakManager) IsLanguagePackDownloaded(langCode string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	pack, exists := em.packs[langCode]
	return exists && pack.Downloaded
}

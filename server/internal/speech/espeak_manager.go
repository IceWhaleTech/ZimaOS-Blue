package speech

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

// EspeakLanguagePack represents an eSpeak-NG language dictionary.
type EspeakLanguagePack struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Downloaded bool   `json:"downloaded"`
	Size       int64  `json:"size"` // actual file size in bytes
}

// EspeakManager checks the real espeak-ng-data directory for available
// language dictionaries. The eSpeak-NG engine is statically linked via
// CGO, so there is no library to download — only the data directory
// needs to be present at runtime.
type EspeakManager struct {
	dataPath string // path to the directory containing espeak-ng-data/
	mu       sync.RWMutex
}

// knownLanguages maps dictionary file prefixes to human-readable names.
var knownLanguages = map[string]string{
	"af": "Afrikaans", "am": "Amharic", "an": "Aragonese",
	"ar": "Arabic", "as": "Assamese", "az": "Azerbaijani",
	"ba": "Bashkir", "be": "Belarusian", "bg": "Bulgarian",
	"bn": "Bengali", "bs": "Bosnian", "ca": "Catalan",
	"cmn": "Chinese (Mandarin)", "cs": "Czech", "cy": "Welsh",
	"da": "Danish", "de": "German", "el": "Greek",
	"en": "English", "eo": "Esperanto", "es": "Spanish",
	"et": "Estonian", "eu": "Basque", "fa": "Persian",
	"fi": "Finnish", "fr": "French", "ga": "Irish",
	"gd": "Scottish Gaelic", "gu": "Gujarati", "he": "Hebrew",
	"hi": "Hindi", "hr": "Croatian", "hu": "Hungarian",
	"hy": "Armenian", "id": "Indonesian", "is": "Icelandic",
	"it": "Italian", "ja": "Japanese", "ka": "Georgian",
	"kk": "Kazakh", "kn": "Kannada", "ko": "Korean",
	"ku": "Kurdish", "ky": "Kyrgyz", "la": "Latin",
	"lt": "Lithuanian", "lv": "Latvian", "mk": "Macedonian",
	"ml": "Malayalam", "mn": "Mongolian", "mr": "Marathi",
	"ms": "Malay", "mt": "Maltese", "my": "Burmese",
	"ne": "Nepali", "nl": "Dutch", "no": "Norwegian",
	"or": "Odia", "pa": "Punjabi", "pl": "Polish",
	"pt": "Portuguese", "ro": "Romanian", "ru": "Russian",
	"si": "Sinhala", "sk": "Slovak", "sl": "Slovenian",
	"sq": "Albanian", "sr": "Serbian", "sv": "Swedish",
	"sw": "Swahili", "ta": "Tamil", "te": "Telugu",
	"th": "Thai", "tr": "Turkish", "tt": "Tatar",
	"uk": "Ukrainian", "ur": "Urdu", "uz": "Uzbek",
	"vi": "Vietnamese",
}

// NewEspeakManager creates a new EspeakManager that detects the real
// espeak-ng-data directory using the same search logic as the CGO provider.
func NewEspeakManager(dataDir string) *EspeakManager {
	em := &EspeakManager{}
	em.dataPath = em.findDataPath()
	return em
}

// findDataPath searches for the espeak-ng-data directory in common locations.
func (em *EspeakManager) findDataPath() string {
	// Check environment variable first
	if envPath := os.Getenv("ESPEAK_DATA_PATH"); envPath != "" {
		if isValidEspeakData(envPath) {
			return envPath
		}
	}

	// Get executable directory
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidates := []string{
			filepath.Join(execDir, "espeak-ng-data"),
			filepath.Join(execDir, "..", "third_party", "espeak-ng", "build", "espeak-ng-data"),
			filepath.Join(execDir, "..", "..", "third_party", "espeak-ng", "build", "espeak-ng-data"),
		}
		for _, path := range candidates {
			if isValidEspeakData(path) {
				return path
			}
		}
	}

	// System paths
	systemPaths := []string{
		"/usr/share/espeak-ng-data",
		"/usr/local/share/espeak-ng-data",
		"/opt/homebrew/share/espeak-ng-data",
	}
	for _, path := range systemPaths {
		if isValidEspeakData(path) {
			return path
		}
	}

	return ""
}

// isValidEspeakData checks if a directory contains the phontab file.
func isValidEspeakData(path string) bool {
	_, err := os.Stat(filepath.Join(path, "phontab"))
	return err == nil
}

// IsLibraryInstalled returns true if the espeak-ng-data directory was found.
// The engine itself is statically linked; this checks for runtime data.
func (em *EspeakManager) IsLibraryInstalled() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.dataPath != ""
}

// GetLibraryPath returns the detected espeak-ng-data path.
func (em *EspeakManager) GetLibraryPath() string {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.dataPath
}

// ListLanguagePacks scans the espeak-ng-data directory for *_dict files
// and returns the list with real file sizes.
func (em *EspeakManager) ListLanguagePacks() []*EspeakLanguagePack {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.dataPath == "" {
		return nil
	}

	entries, err := os.ReadDir(em.dataPath)
	if err != nil {
		return nil
	}

	var packs []*EspeakLanguagePack
	for _, entry := range entries {
		name := entry.Name()
		// Match *_dict files
		if len(name) < 5 || name[len(name)-5:] != "_dict" {
			continue
		}
		code := name[:len(name)-5]
		langName := knownLanguages[code]
		if langName == "" {
			langName = code // fallback to code
		}

		var size int64
		if info, err := entry.Info(); err == nil {
			size = info.Size()
		}

		packs = append(packs, &EspeakLanguagePack{
			Code:       code,
			Name:       langName,
			Downloaded: true, // if the file exists, it's available
			Size:       size,
		})
	}

	sort.Slice(packs, func(i, j int) bool {
		return packs[i].Code < packs[j].Code
	})
	return packs
}

// GetDataSize returns the total size of the espeak-ng-data directory in bytes.
func (em *EspeakManager) GetDataSize() int64 {
	em.mu.RLock()
	path := em.dataPath
	em.mu.RUnlock()

	if path == "" {
		return 0
	}

	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// LanguageCount returns the number of *_dict files found.
func (em *EspeakManager) LanguageCount() int {
	packs := em.ListLanguagePacks()
	return len(packs)
}

// Refresh re-scans for the espeak-ng-data directory.
func (em *EspeakManager) Refresh() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.dataPath = em.findDataPath()
}

// GetPlatformInfo returns OS/arch info (useful for status display).
func (em *EspeakManager) GetPlatformInfo() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}

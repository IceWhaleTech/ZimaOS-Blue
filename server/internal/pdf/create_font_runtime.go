package pdf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

var (
	createFontRuntimeMu        sync.Mutex
	createFontRuntimeDirsKey   string
	createFontRuntimeFileIndex map[string][]string
)

func collectCreateRequiredRunes(texts []string) map[rune]struct{} {
	required := make(map[rune]struct{})
	for _, text := range texts {
		for _, r := range text {
			switch r {
			case '\n', '\r', '\t', '\u00a0':
				continue
			}
			if r >= 32 && r <= 126 {
				continue
			}
			if _, ok := createRuneFallback(r); ok {
				continue
			}
			required[r] = struct{}{}
		}
	}
	return required
}

func resolveCreateUnicodeFont(required map[rune]struct{}) ([]byte, error) {
	if len(required) == 0 {
		return nil, nil
	}

	index, err := createFontRuntimeFiles()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, path := range prioritizedCreateFontPaths(index) {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		data, ok, err := createFontFileSupports(path, required)
		if err == nil && ok {
			return data, nil
		}
	}

	return nil, fmt.Errorf("no unicode TTF font found with required glyph coverage")
}

func createFontSupportFunc(fontBytes []byte) (func(rune) bool, error) {
	if len(fontBytes) == 0 {
		return nil, nil
	}
	font, err := sfnt.Parse(fontBytes)
	if err != nil {
		return nil, err
	}

	var buf sfnt.Buffer
	return func(r rune) bool {
		if r >= 32 && r <= 126 {
			return true
		}
		index, err := font.GlyphIndex(&buf, r)
		return err == nil && index != 0
	}, nil
}

func createFontRuntimeFiles() (map[string][]string, error) {
	dirs := createFontSearchDirs()
	key := strings.Join(dirs, "\n")

	createFontRuntimeMu.Lock()
	if createFontRuntimeFileIndex != nil && createFontRuntimeDirsKey == key {
		index := createFontRuntimeFileIndex
		createFontRuntimeMu.Unlock()
		return index, nil
	}
	createFontRuntimeMu.Unlock()

	index := make(map[string][]string)
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d == nil || d.IsDir() {
				return nil
			}
			if strings.ToLower(filepath.Ext(d.Name())) != ".ttf" {
				return nil
			}
			base := strings.ToLower(d.Name())
			index[base] = append(index[base], path)
			return nil
		})
	}

	for base := range index {
		sort.Strings(index[base])
	}

	createFontRuntimeMu.Lock()
	createFontRuntimeDirsKey = key
	createFontRuntimeFileIndex = index
	createFontRuntimeMu.Unlock()
	return index, nil
}

func prioritizedCreateFontPaths(index map[string][]string) []string {
	priorityBaseNames := []string{
		"arial unicode.ttf",
		"arialuni.ttf",
		"msyh.ttf",
		"microsoftyahei.ttf",
		"simhei.ttf",
		"simsun.ttf",
		"noto sans cjk sc regular.ttf",
		"notosanssc-regular.ttf",
		"sourcehansanssc-regular.ttf",
		"wqy-zenhei.ttf",
	}

	paths := make([]string, 0, 64)
	seen := make(map[string]struct{})
	appendPath := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	for _, base := range priorityBaseNames {
		for _, path := range index[base] {
			appendPath(path)
		}
	}

	remainingBaseNames := make([]string, 0, len(index))
	for base := range index {
		remainingBaseNames = append(remainingBaseNames, base)
	}
	sort.Strings(remainingBaseNames)
	for _, base := range remainingBaseNames {
		for _, path := range index[base] {
			appendPath(path)
		}
	}

	return paths
}

func createFontFileSupports(path string, required map[rune]struct{}) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		return nil, false, err
	}

	var buf sfnt.Buffer
	for r := range required {
		index, glyphErr := font.GlyphIndex(&buf, r)
		if glyphErr != nil || index == 0 {
			return nil, false, nil
		}
	}
	return data, true, nil
}

func createFontSearchDirs() []string {
	dirs := make([]string, 0, 8)
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs, "/System/Library/Fonts", "/Library/Fonts")
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
	case "linux":
		dirs = append(dirs, "/usr/share/fonts", "/usr/local/share/fonts")
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dirs = append(dirs, filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts"))
		}
	case "windows":
		if windir := strings.TrimSpace(os.Getenv("WINDIR")); windir != "" {
			dirs = append(dirs, filepath.Join(windir, "Fonts"))
		}
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		result = append(result, dir)
	}
	return result
}

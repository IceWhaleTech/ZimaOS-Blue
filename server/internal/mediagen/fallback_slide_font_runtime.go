package mediagen

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

type slideFontRuntimeRequest struct {
	family string
	weight string
}

var (
	slideFontRuntimeMu       sync.Mutex
	slideFontRuntimeDataDir  string
	slideFontRuntimeDirsKey  string
	slideFontRuntimeFileIndex map[string][]string
)

func configureSlideFontRuntimeDataDir(dataDir string) {
	dataDir = strings.TrimSpace(dataDir)

	slideFontRuntimeMu.Lock()
	defer slideFontRuntimeMu.Unlock()

	if slideFontRuntimeDataDir == dataDir {
		return
	}
	slideFontRuntimeDataDir = dataDir
	slideFontRuntimeDirsKey = ""
	slideFontRuntimeFileIndex = nil
	slideFontRegistry = map[string]*opentype.Font{}
	slideFontErrors = map[string]error{}
}

func canonicalSlideFontRuntimeRequest(family, weight string) slideFontRuntimeRequest {
	req := slideFontRuntimeRequest{
		family: normalizeSlideFontFamily(family),
		weight: normalizeSlideFontWeight(weight),
	}

	switch req.family {
	case "display":
		req.family = "sans"
		req.weight = "bold"
	case "mono":
		switch req.weight {
		case "bold", "bold_italic":
			req.weight = "bold"
		case "medium", "medium_italic":
			req.weight = "bold"
		case "italic":
			req.weight = "regular"
		case "regular":
			return req
		default:
			req.weight = "regular"
		}
	default:
		switch req.weight {
		case "italic":
			return req
		case "medium", "bold", "medium_italic", "bold_italic":
			req.weight = "bold"
		default:
			req.weight = "regular"
		}
	}

	return req
}

func loadSlideRuntimeFont(family, weight string) (*opentype.Font, error) {
	path, faceIndex, err := resolveSlideFontRuntimeSource(family, weight)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read slide font %q: %w", path, err)
	}
	collection, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse slide font %q: %w", path, err)
	}
	font, err := collection.Font(faceIndex)
	if err != nil {
		return nil, fmt.Errorf("load slide font %q face %d: %w", path, faceIndex, err)
	}
	return font, nil
}

func resolveSlideFontRuntimeSource(family, weight string) (string, int, error) {
	req := canonicalSlideFontRuntimeRequest(family, weight)

	index, err := slideFontRuntimeFiles()
	if err != nil {
		return "", 0, err
	}

	for _, baseName := range slideFontCandidateBaseNames(req) {
		paths := index[strings.ToLower(baseName)]
		for _, path := range paths {
			faceIndex, err := bestSlideFontFaceIndex(path, req)
			if err == nil {
				return path, faceIndex, nil
			}
		}
	}

	return "", 0, fmt.Errorf("no runtime font found for %s:%s", req.family, req.weight)
}

func slideFontRuntimeFiles() (map[string][]string, error) {
	dirs := slideFontSearchDirs()
	key := strings.Join(dirs, "\n")

	slideFontRuntimeMu.Lock()
	if slideFontRuntimeFileIndex != nil && slideFontRuntimeDirsKey == key {
		index := slideFontRuntimeFileIndex
		slideFontRuntimeMu.Unlock()
		return index, nil
	}
	slideFontRuntimeMu.Unlock()

	index := map[string][]string{}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			switch ext {
			case ".ttf", ".ttc", ".otf":
				base := strings.ToLower(d.Name())
				index[base] = append(index[base], path)
			}
			return nil
		})
	}

	slideFontRuntimeMu.Lock()
	slideFontRuntimeDirsKey = key
	slideFontRuntimeFileIndex = index
	slideFontRuntimeMu.Unlock()
	return index, nil
}

func slideFontSearchDirs() []string {
	slideFontRuntimeMu.Lock()
	dataDir := slideFontRuntimeDataDir
	slideFontRuntimeMu.Unlock()

	dirs := make([]string, 0, 10)
	if dataDir != "" {
		dirs = append(dirs,
			filepath.Join(dataDir, "media", "fonts"),
			filepath.Join(dataDir, "fonts"),
		)
	}

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

	seen := map[string]struct{}{}
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

func slideFontCandidateBaseNames(req slideFontRuntimeRequest) []string {
	exact := func(name string) []string {
		return []string{name + ".ttf", name + ".ttc", name + ".otf"}
	}

	var names []string
	names = append(names, exact("slide-"+req.family+"-"+req.weight)...)
	names = append(names, exact(req.family+"-"+req.weight)...)

	switch req.family {
	case "mono":
		switch req.weight {
		case "bold":
			names = append(names,
				"SFNSMono.ttf",
				"Menlo.ttc",
				"Courier.ttc",
				"Monaco.ttf",
				"DejaVuSansMono-Bold.ttf",
				"LiberationMono-Bold.ttf",
				"NotoSansMono-Bold.ttf",
				"mono-regular.ttf",
			)
		default:
			names = append(names,
				"SFNSMono.ttf",
				"Menlo.ttc",
				"Courier.ttc",
				"Monaco.ttf",
				"DejaVuSansMono.ttf",
				"LiberationMono-Regular.ttf",
				"NotoSansMono-Regular.ttf",
			)
		}
	default:
		switch req.weight {
		case "bold":
			names = append(names,
				"SFCompact.ttf",
				"SFNS.ttf",
				"HelveticaNeue.ttc",
				"Helvetica.ttc",
				"Avenir Next.ttc",
				"Avenir.ttc",
				"Arial Bold.ttf",
				"ArialHB.ttc",
				"Verdana Bold.ttf",
				"Trebuchet MS Bold.ttf",
				"DejaVuSans-Bold.ttf",
				"LiberationSans-Bold.ttf",
				"NotoSans-Bold.ttf",
				"sans-regular.ttf",
			)
		case "italic":
			names = append(names,
				"SFNSItalic.ttf",
				"SFCompactItalic.ttf",
				"HelveticaNeue.ttc",
				"Helvetica.ttc",
				"Arial Italic.ttf",
				"Verdana Italic.ttf",
				"Trebuchet MS Italic.ttf",
				"DejaVuSans-Oblique.ttf",
				"LiberationSans-Italic.ttf",
				"NotoSans-Italic.ttf",
				"sans-regular.ttf",
			)
		default:
			names = append(names,
				"SFNS.ttf",
				"HelveticaNeue.ttc",
				"Helvetica.ttc",
				"Avenir Next.ttc",
				"Avenir.ttc",
				"Arial Unicode.ttf",
				"Arial.ttf",
				"Verdana.ttf",
				"Trebuchet MS.ttf",
				"DejaVuSans.ttf",
				"LiberationSans-Regular.ttf",
				"NotoSans-Regular.ttf",
			)
		}
	}

	return names
}

func bestSlideFontFaceIndex(path string, req slideFontRuntimeRequest) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	collection, err := opentype.ParseCollection(data)
	if err != nil {
		return 0, err
	}

	bestIndex := 0
	bestScore := -1
	for i := 0; i < collection.NumFonts(); i++ {
		font, err := collection.Font(i)
		if err != nil || font == nil {
			continue
		}
		score := slideFontFaceScore(font, req)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestScore < 0 {
		return 0, fmt.Errorf("no usable font face in %q", path)
	}
	return bestIndex, nil
}

func slideFontFaceScore(font *sfnt.Font, req slideFontRuntimeRequest) int {
	names := slideFontFaceNames(font)
	joined := strings.ToLower(strings.Join(names, " "))
	score := 0

	switch req.family {
	case "mono":
		if containsAny(joined, "mono", "menlo", "courier", "monaco", "console", "code") {
			score += 8
		}
		if containsAny(joined, "sans", "arial", "helvetica", "avenir") {
			score -= 4
		}
	default:
		if containsAny(joined, "sans", "arial", "helvetica", "avenir", "sf", "verdana", "dejavu", "liberation", "noto") {
			score += 6
		}
		if containsAny(joined, "mono", "courier", "menlo", "monaco") {
			score -= 6
		}
		if containsAny(joined, "serif", "times", "georgia", "bodoni", "palatino") {
			score -= 3
		}
	}

	switch req.weight {
	case "bold":
		if containsAny(joined, "bold", "semibold", "demibold", "medium") {
			score += 4
		}
		if containsAny(joined, "light", "thin") {
			score -= 2
		}
	case "italic":
		if containsAny(joined, "italic", "oblique") {
			score += 4
		}
	default:
		if containsAny(joined, "regular", "book", "roman", "normal") {
			score += 3
		}
	}

	return score
}

func slideFontFaceNames(font *sfnt.Font) []string {
	if font == nil {
		return nil
	}
	var names []string
	var buf sfnt.Buffer
	for _, id := range []sfnt.NameID{
		sfnt.NameIDFamily,
		sfnt.NameIDSubfamily,
		sfnt.NameIDTypographicFamily,
		sfnt.NameIDTypographicSubfamily,
		sfnt.NameIDFull,
	} {
		name, err := font.Name(&buf, id)
		if err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

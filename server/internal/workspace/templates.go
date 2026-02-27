package workspace

import (
	"embed"
	"io/fs"
	"strings"
	"sync"
)

//go:embed templates/*/*.md
var templatesFS embed.FS

// templateSet holds all default templates for a single locale.
type templateSet struct {
	soul      string
	user      string
	identity  string
	agents    string
	memory    string
	heartbeat string
	bootstrap string
}

// localeAliases maps locale keys that share templates with another locale directory.
var localeAliases = map[string]string{
	"zh-HK": "zh-TW",
}

var (
	templateCache   = map[string]*templateSet{}
	templateCacheMu sync.RWMutex
)

// loadTemplateSet reads all .md files from templates/<locale>/ in the embedded FS.
func loadTemplateSet(locale string) *templateSet {
	ts := &templateSet{}
	dir := "templates/" + locale
	entries, err := fs.ReadDir(templatesFS, dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(templatesFS, dir+"/"+e.Name())
		if err != nil {
			continue
		}
		content := string(data)
		switch e.Name() {
		case "SOUL.md":
			ts.soul = content
		case "USER.md":
			ts.user = content
		case "IDENTITY.md":
			ts.identity = content
		case "AGENTS.md":
			ts.agents = content
		case "MEMORY.md":
			ts.memory = content
		case "HEARTBEAT.md":
			ts.heartbeat = content
		case "BOOTSTRAP.md":
			ts.bootstrap = content
		}
	}
	return ts
}

// availableLocales returns all locale directory names from the embedded FS.
func availableLocales() []string {
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		return nil
	}
	var locales []string
	for _, e := range entries {
		if e.IsDir() {
			locales = append(locales, e.Name())
		}
	}
	return locales
}

// getTemplates returns the template set for the given locale, falling back to English.
// Lookup order: exact match (e.g. "zh-TW") → alias → language prefix (e.g. "zh") → "en".
func getTemplates(locale string) *templateSet {
	// Check cache first
	templateCacheMu.RLock()
	if ts, ok := templateCache[locale]; ok {
		templateCacheMu.RUnlock()
		return ts
	}
	templateCacheMu.RUnlock()

	ts := resolveTemplates(locale)

	templateCacheMu.Lock()
	templateCache[locale] = ts
	templateCacheMu.Unlock()
	return ts
}

// resolveTemplates tries to find the best matching template set for a locale.
func resolveTemplates(locale string) *templateSet {
	// 1. Exact match
	if ts := loadTemplateSet(locale); ts != nil {
		return ts
	}
	// 2. Alias (e.g. "zh-HK" → "zh-TW")
	if alias, ok := localeAliases[locale]; ok {
		if ts := loadTemplateSet(alias); ts != nil {
			return ts
		}
	}
	// 3. Try uppercase region: "zh-tw" → "zh-TW"
	if idx := strings.IndexByte(locale, '-'); idx > 0 && idx+1 < len(locale) {
		normalized := locale[:idx] + "-" + strings.ToUpper(locale[idx+1:])
		if ts := loadTemplateSet(normalized); ts != nil {
			return ts
		}
		// Check alias for normalized form too
		if alias, ok := localeAliases[normalized]; ok {
			if ts := loadTemplateSet(alias); ts != nil {
				return ts
			}
		}
	}
	// 4. Language prefix: "zh-CN" → "zh"
	if len(locale) >= 2 {
		if ts := loadTemplateSet(locale[:2]); ts != nil {
			return ts
		}
	}
	// 5. Fallback to English
	ts := loadTemplateSet("en")
	if ts == nil {
		// Should never happen — en templates are always embedded
		return &templateSet{}
	}
	return ts
}

// templateMap returns the standard file→content map (excluding bootstrap).
func (ts *templateSet) templateMap() map[string]string {
	return map[string]string{
		FileSOUL:      ts.soul,
		FileUSER:      ts.user,
		FileIDENTITY:  ts.identity,
		FileAGENTS:    ts.agents,
		FileMEMORY:    ts.memory,
		FileHEARTBEAT: ts.heartbeat,
	}
}

// templateContentByFile returns the template content for a known workspace file.
func templateContentByFile(ts *templateSet, name string) string {
	if ts == nil {
		return ""
	}
	switch name {
	case FileSOUL:
		return ts.soul
	case FileUSER:
		return ts.user
	case FileIDENTITY:
		return ts.identity
	case FileAGENTS:
		return ts.agents
	case FileMEMORY:
		return ts.memory
	case FileHEARTBEAT:
		return ts.heartbeat
	case FileBOOTSTRAP:
		return ts.bootstrap
	default:
		return ""
	}
}

// NormalizeDefaultTemplateToEnglish maps locale-specific default workspace template
// content to its English template equivalent. Non-default or unknown content is returned unchanged.
func NormalizeDefaultTemplateToEnglish(name, content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return content
	}

	en := strings.TrimSpace(templateContentByFile(getTemplates("en"), name))
	if en == "" {
		return content
	}

	for _, locale := range availableLocales() {
		ts := getTemplates(locale)
		if strings.TrimSpace(templateContentByFile(ts, name)) == trimmed {
			return en
		}
	}
	return content
}

package skillmanifest

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	skillembed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/embedded"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
)

var (
	skillScriptPathRegexpOnce sync.Once
	skillScriptPathRegexp     *regexp.Regexp
	nonSlugCharsOnce          sync.Once
	nonSlugChars              *regexp.Regexp
)

func ensureSkillScriptPathRegexp() {
	skillScriptPathRegexpOnce.Do(func() {
		skillScriptPathRegexp = regexp.MustCompile(`(?i)(?:^|[\s` + "`" + `\(\[])((?:\./)?scripts/[a-z0-9._/\-]+)`)
	})
}

func ensureNonSlugChars() {
	nonSlugCharsOnce.Do(func() {
		nonSlugChars = regexp.MustCompile(`[^a-z0-9._-]+`)
	})
}

const (
	ManifestMetadataContractStatus = "contract_status"
	ManifestMetadataContractSource = "contract_source"
	ManifestMetadataContractNotes  = "contract_notes"

	ContractStatusStrict         = "strict_contract"
	ContractStatusLegacyFallback = "legacy_fallback"
	ContractStatusGenerated      = "generated_contract"

	ContractSourceDeclaredFrontmatter   = "declared_frontmatter"
	ContractSourceLegacyFrontmatter     = "legacy_frontmatter_fallback"
	ContractSourceGeneratedSafeDefaults = "generated_safe_defaults"
)

type Route struct {
	Intent string
	Action string
}

type ErrorRule struct {
	Error      string
	Resolution string
}

type Options struct {
	RequireContract     bool
	AllowLegacyFallback bool
}

type Document struct {
	ID              string
	Name            string
	Description     string
	Version         string
	Author          string
	Category        string
	Invocation      string
	Examples        []string
	CapabilityTags  []string
	Paths           []string
	UserInvocable   bool
	ModelInvocable  bool
	InteractionMode string
	CardSupport     string
	Permissions     []string
	Location        string
	EntryFile       string
	Enabled         bool
	OS              []string
	Environment     []string
	Tags            []string
	Setup           string
	ScriptPaths     []string
	InstallSteps    []string
	UsageSteps      []string
	TaskRoutes      []Route
	ErrorRules      []ErrorRule
	Example         string
	Body            string
	Manifest        *skill.Manifest
	ValidationNotes []string
}

type ValidationError struct {
	Issues []string
}

type contractMetadata struct {
	status string
	source string
	notes  []string
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "skill frontmatter validation failed"
	}
	return "skill frontmatter validation failed: " + strings.Join(e.Issues, "; ")
}

func ResolveRoots(workspaceDir string) []string {
	var roots []string
	seen := map[string]struct{}{}

	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		roots = append(roots, dir)
	}

	if workspaceDir != "" {
		add(filepath.Join(workspaceDir, ".agents", "skills"))
		add(filepath.Join(workspaceDir, ".claude", "skills"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".agents", "skills"))
		add(filepath.Join(home, ".claude", "skills"))
	}
	return roots
}

// ResolvePeerRootsForManagedDir returns the peer .agents/.claude skill roots
// for a managed skills directory when it follows the conventional
// {scope}/.{agents|claude}/skills layout. The returned roots preserve the
// canonical precedence order: .agents before .claude. If the directory does not
// match a known managed layout, the directory itself is returned as the only
// root.
func ResolvePeerRootsForManagedDir(skillsDir string) []string {
	skillsDir = strings.TrimSpace(skillsDir)
	if skillsDir == "" {
		return nil
	}

	cleanDir := filepath.Clean(skillsDir)
	added := make(map[string]struct{}, 2)
	roots := make([]string, 0, 2)
	add := func(root string) {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" {
			return
		}
		if _, ok := added[root]; ok {
			return
		}
		added[root] = struct{}{}
		roots = append(roots, root)
	}

	if filepath.Base(cleanDir) != "skills" {
		add(cleanDir)
		return roots
	}

	containerDir := filepath.Dir(cleanDir)
	switch filepath.Base(containerDir) {
	case ".agents", ".claude":
		scopeDir := filepath.Dir(containerDir)
		add(filepath.Join(scopeDir, ".agents", "skills"))
		add(filepath.Join(scopeDir, ".claude", "skills"))
	default:
		add(cleanDir)
	}

	return roots
}

func ReadDir(skillDir string, opts Options) (Document, []byte, error) {
	entryDoc, err := skillbundle.FindEntryDocumentInDir(skillDir)
	if err != nil {
		return Document{}, nil, err
	}
	data, err := os.ReadFile(entryDoc.Path)
	if err != nil {
		return Document{}, nil, err
	}
	doc, err := ParseEntry(filepath.Base(skillDir), entryDoc.Path, data, opts)
	if err != nil {
		return Document{}, nil, err
	}
	return doc, data, nil
}

func ReadEmbedded(skillID string, opts Options) (Document, []byte, error) {
	embedPath := path.Join("skills", strings.TrimSpace(skillID), "SKILL.md")
	data, err := fs.ReadFile(skillembed.SkillsFS, embedPath)
	if err != nil {
		return Document{}, nil, err
	}
	doc, err := ParseEntry(skillID, "embedded:"+embedPath, data, opts)
	if err != nil {
		return Document{}, nil, err
	}
	return doc, data, nil
}

func ListEmbeddedIDs() ([]string, error) {
	entries, err := fs.ReadDir(skillembed.SkillsFS, "skills")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ids = append(ids, entry.Name())
	}
	sort.Strings(ids)
	return ids, nil
}

func ParseEntry(dirName, sourcePath string, data []byte, opts Options) (Document, error) {
	content := string(data)
	meta, body, err := splitFrontmatter(content)
	if err != nil {
		return Document{}, err
	}

	doc := Document{
		Name:           strings.TrimSpace(dirName),
		Location:       sourcePath,
		EntryFile:      filepath.Base(sourcePath),
		Enabled:        true,
		UserInvocable:  true,
		ModelInvocable: true,
		Body:           strings.TrimSpace(body),
	}

	parseFrontmatter(meta, &doc)
	parseBody(doc.Body, &doc)

	if doc.Description == "" {
		doc.Description = extractFirstParagraph(doc.Body)
	}
	if doc.Name == "" {
		doc.Name = strings.TrimSpace(dirName)
	}
	doc.ID = normalizeSkillID(firstNonBlank(doc.ID, doc.Name, dirName))

	strictIssues := validateStrictContract(meta, &doc)
	legacyFallbackApplied := false
	if opts.RequireContract {
		if len(strictIssues) > 0 {
			if opts.AllowLegacyFallback && canUseLegacyContractFallback(meta, &doc, strictIssues) {
				legacyFallbackApplied = true
				doc.ValidationNotes = appendUnique(doc.ValidationNotes, legacyContractValidationNote(strictIssues))
				repairLegacyContractFields(&doc, strictIssues)
				deriveLegacyDefaults(&doc)
			} else {
				return Document{}, &ValidationError{Issues: strictIssues}
			}
		}
	} else {
		if len(strictIssues) > 0 && canUseLegacyContractFallback(meta, &doc, strictIssues) {
			legacyFallbackApplied = true
			doc.ValidationNotes = appendUnique(doc.ValidationNotes, legacyContractValidationNote(strictIssues))
			repairLegacyContractFields(&doc, strictIssues)
		}
		deriveLegacyDefaults(&doc)
	}
	contractMeta := deriveContractMetadata(meta, strictIssues, legacyFallbackApplied)

	if shouldPromoteIDToName(meta, doc.Name, dirName) && doc.ID != "" {
		doc.Name = doc.ID
	}
	if doc.Name == "" {
		doc.Name = doc.ID
	}
	if doc.Version == "" {
		doc.Version = "0.1.0"
	}
	if doc.Description == "" {
		doc.Description = doc.Name
	}
	if doc.Invocation == "" {
		doc.Invocation = "blue " + doc.ID
	}
	if len(doc.Examples) == 0 {
		doc.Examples = []string{doc.Invocation}
	}
	if len(doc.CapabilityTags) == 0 {
		doc.CapabilityTags = append([]string(nil), doc.Tags...)
	}
	if len(doc.CapabilityTags) == 0 {
		doc.CapabilityTags = []string{defaultCapabilityTag(&doc)}
	}
	if doc.InteractionMode == "" {
		doc.InteractionMode = "stateless"
	}
	if doc.CardSupport == "" {
		doc.CardSupport = "none"
	}

	doc.Tags = appendUnique(doc.Tags, doc.CapabilityTags...)
	doc.Manifest = &skill.Manifest{
		ID:              doc.ID,
		Name:            doc.Name,
		Version:         doc.Version,
		Description:     doc.Description,
		Author:          doc.Author,
		Category:        doc.Category,
		Tags:            append([]string(nil), doc.Tags...),
		Paths:           append([]string(nil), doc.Paths...),
		UserInvocable:   doc.UserInvocable,
		ModelInvocable:  doc.ModelInvocable,
		Permissions:     append([]string(nil), doc.Permissions...),
		Invocation:      doc.Invocation,
		Examples:        append([]string(nil), doc.Examples...),
		CapabilityTags:  append([]string(nil), doc.CapabilityTags...),
		InteractionMode: doc.InteractionMode,
		CardSupport:     doc.CardSupport,
		Metadata: map[string]string{
			"source_path": doc.Location,
			"entry_file":  doc.EntryFile,
		},
	}
	if contractMeta.status != "" {
		doc.Manifest.Metadata[ManifestMetadataContractStatus] = contractMeta.status
	}
	if contractMeta.source != "" {
		doc.Manifest.Metadata[ManifestMetadataContractSource] = contractMeta.source
	}
	if len(contractMeta.notes) > 0 {
		doc.Manifest.Metadata[ManifestMetadataContractNotes] = strings.Join(contractMeta.notes, "\n")
	}
	if len(doc.ValidationNotes) > 0 {
		doc.Manifest.Metadata["validation_notes"] = strings.Join(doc.ValidationNotes, "\n")
	}
	return doc, nil
}

func PlatformMatch(osList []string) bool {
	if len(osList) == 0 {
		return true
	}
	for _, candidate := range osList {
		if strings.EqualFold(strings.TrimSpace(candidate), runtime.GOOS) {
			return true
		}
	}
	return false
}

func splitFrontmatter(content string) (map[string]any, string, error) {
	body := strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, body, nil
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, body, nil
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return meta, strings.TrimSpace(parts[2]), nil
}

func canUseLegacyContractFallback(meta map[string]any, doc *Document, issues []string) bool {
	if len(meta) == 0 || doc == nil {
		return false
	}
	if strings.TrimSpace(doc.Description) == "" {
		return false
	}
	explicitID := strings.TrimSpace(stringValue(meta["id"]))
	explicitName := strings.TrimSpace(stringValue(meta["name"]))
	explicitDescription := strings.TrimSpace(stringValue(meta["description"]))
	if explicitID == "" && explicitName == "" && explicitDescription == "" {
		return false
	}
	resolvedID := normalizeSkillID(firstNonBlank(doc.ID, doc.Name))
	if isGenericDerivedInstallID(resolvedID) && explicitID == "" && explicitName == "" {
		return false
	}
	for _, issue := range issues {
		lower := strings.ToLower(strings.TrimSpace(issue))
		if strings.Contains(lower, "frontmatter field name must be a string") ||
			strings.Contains(lower, "frontmatter field description must be a string") {
			return false
		}
	}
	return true
}

func legacyContractValidationNote(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	return "legacy manifest compatibility fallback applied: " + strings.Join(issues, "; ")
}

func generatedContractValidationNote(meta map[string]any, issues []string) string {
	base := "generated safe structural contract defaults because strict frontmatter contract was missing"
	if len(meta) > 0 {
		base = "generated safe structural contract defaults because strict frontmatter contract was incomplete or invalid"
	}
	if len(issues) == 0 {
		return base
	}
	return base + ": " + strings.Join(issues, "; ")
}

func deriveContractMetadata(meta map[string]any, strictIssues []string, legacyFallbackApplied bool) contractMetadata {
	switch {
	case len(strictIssues) == 0:
		return contractMetadata{
			status: ContractStatusStrict,
			source: ContractSourceDeclaredFrontmatter,
		}
	case legacyFallbackApplied:
		return contractMetadata{
			status: ContractStatusLegacyFallback,
			source: ContractSourceLegacyFrontmatter,
			notes:  appendUnique(nil, legacyContractValidationNote(strictIssues)),
		}
	default:
		return contractMetadata{
			status: ContractStatusGenerated,
			source: ContractSourceGeneratedSafeDefaults,
			notes:  appendUnique(nil, generatedContractValidationNote(meta, strictIssues)),
		}
	}
}

func repairLegacyContractFields(doc *Document, issues []string) {
	if doc == nil {
		return
	}
	for _, issue := range issues {
		switch {
		case strings.Contains(issue, "missing required frontmatter field: invocation"),
			strings.Contains(issue, "frontmatter field invocation must be a string"),
			strings.Contains(issue, "frontmatter invocation must be a parseable `blue ...` command"):
			doc.Invocation = ""
		case strings.Contains(issue, "missing required frontmatter field: examples"),
			strings.Contains(issue, "frontmatter field examples must be a YAML list of strings"),
			strings.Contains(issue, "frontmatter examples must be parseable `blue ...` commands"):
			doc.Examples = nil
		case strings.Contains(issue, "missing required frontmatter field: capability_tags"),
			strings.Contains(issue, "frontmatter field capability_tags must be a YAML list of strings"):
			doc.CapabilityTags = nil
		case strings.Contains(issue, "missing required frontmatter field: card_support"),
			strings.Contains(issue, "frontmatter field card_support must be one of none, batch, streaming, both"):
			doc.CardSupport = ""
		}
	}
}

func shouldPromoteIDToName(meta map[string]any, currentName, dirName string) bool {
	if strings.TrimSpace(stringValue(meta["name"])) != "" {
		return false
	}
	return isGenericDerivedInstallID(firstNonBlank(currentName, dirName))
}

func isGenericDerivedInstallID(value string) bool {
	switch normalizeSkillID(value) {
	case "", "skill", "claude", "agent":
		return true
	default:
		return false
	}
}

func parseFrontmatter(meta map[string]any, doc *Document) {
	if doc == nil || len(meta) == 0 {
		return
	}
	if value := stringValue(meta["id"]); value != "" {
		doc.ID = value
	}
	if value := stringValue(meta["name"]); value != "" {
		doc.Name = value
	}
	if value := stringValue(meta["description"]); value != "" {
		doc.Description = value
	}
	if value := stringValue(meta["version"]); value != "" {
		doc.Version = value
	}
	if value := stringValue(meta["author"]); value != "" {
		doc.Author = value
	}
	if value := stringValue(meta["category"]); value != "" {
		doc.Category = value
	}
	if value := stringValue(meta["invocation"]); value != "" {
		doc.Invocation = value
	}
	doc.Paths = appendUnique(doc.Paths, stringListValue(meta["paths"])...)
	if value := stringValue(meta["interaction_mode"]); value != "" {
		doc.InteractionMode = value
	}
	if value := normalizeCardSupport(meta["card_support"]); value != "" {
		doc.CardSupport = value
	}
	if enabled, ok := boolValue(meta["enabled"]); ok {
		doc.Enabled = enabled
	}
	if userInvocable, ok := boolValue(firstNonNil(meta["user_invocable"], meta["user-invocable"])); ok {
		doc.UserInvocable = userInvocable
	}
	if modelInvocable, ok := boolValue(meta["model_invocable"]); ok {
		doc.ModelInvocable = modelInvocable
	}
	if disableModel, ok := boolValue(meta["disable-model-invocation"]); ok && disableModel {
		doc.ModelInvocable = false
	}
	doc.OS = appendUnique(doc.OS, stringListValue(meta["os"])...)
	doc.Environment = appendUnique(doc.Environment, stringListValue(firstNonNil(meta["environment"], meta["env"], meta["applicable_env"], meta["applicable-environment"]))...)
	doc.Tags = appendUnique(doc.Tags, stringListValue(meta["tags"])...)
	doc.CapabilityTags = appendUnique(doc.CapabilityTags, stringListValue(meta["capability_tags"])...)
	doc.Examples = appendUnique(doc.Examples, stringListValue(meta["examples"])...)
	doc.Permissions = appendUnique(doc.Permissions, stringListValue(meta["permissions"])...)
}

func parseBody(body string, doc *Document) {
	if doc == nil {
		return
	}
	sections := splitSections(body)

	setup := firstSectionContent(sections,
		"setup", "installation", "install", "环境准备", "安装", "前置条件", "准备")
	if setup != "" {
		doc.Setup = truncateForIndex(stripMarkdownTableRows(setup), 320)
		doc.InstallSteps = extractCommands(setup)
	}

	availableScripts := firstSectionContent(sections,
		"available scripts", "scripts", "脚本", "可用脚本")
	scriptUsage := firstSectionContent(sections,
		"script usage", "command usage", "usage", "命令用法", "脚本用法", "使用脚本", "用法")
	taskRouting := firstSectionContent(sections,
		"task routing", "routing", "intent routing", "任务路由", "任务分流", "路由")
	errorHandling := firstSectionContent(sections,
		"error handling", "errors", "错误处理", "故障处理")

	doc.ScriptPaths = appendUnique(doc.ScriptPaths, extractScriptPaths(availableScripts)...)
	doc.ScriptPaths = appendUnique(doc.ScriptPaths, extractScriptPaths(scriptUsage)...)
	doc.ScriptPaths = appendUnique(doc.ScriptPaths, extractScriptPaths(body)...)
	if scriptUsage != "" {
		doc.UsageSteps = appendUnique(doc.UsageSteps, extractCommands(scriptUsage)...)
	} else {
		doc.UsageSteps = appendUnique(doc.UsageSteps, extractCommands(body)...)
	}
	doc.TaskRoutes = append(doc.TaskRoutes, parseTaskRoutes(taskRouting)...)
	doc.ErrorRules = append(doc.ErrorRules, parseErrorRules(errorHandling)...)

	if doc.Example == "" {
		if len(doc.Examples) > 0 {
			doc.Example = truncateForIndex(doc.Examples[0], 180)
		} else if len(doc.UsageSteps) > 0 {
			doc.Example = truncateForIndex(doc.UsageSteps[0], 180)
		} else {
			doc.Example = extractSkillExample(body, firstNonBlank(doc.ID, doc.Name))
		}
	}
}

func deriveLegacyDefaults(doc *Document) {
	if doc == nil {
		return
	}
	if doc.Version == "" {
		doc.Version = "0.1.0"
	}
	if doc.Invocation == "" {
		if len(doc.UsageSteps) > 0 {
			doc.Invocation = doc.UsageSteps[0]
		} else {
			doc.Invocation = "blue " + firstNonBlank(doc.ID, doc.Name)
		}
	}
	if len(doc.Examples) == 0 {
		switch {
		case len(doc.UsageSteps) > 0:
			doc.Examples = append(doc.Examples, doc.UsageSteps...)
		case doc.Example != "":
			doc.Examples = append(doc.Examples, doc.Example)
		case doc.Invocation != "":
			doc.Examples = append(doc.Examples, doc.Invocation)
		}
	}
	if len(doc.CapabilityTags) == 0 {
		doc.CapabilityTags = append(doc.CapabilityTags, doc.Tags...)
	}
	if len(doc.CapabilityTags) == 0 {
		doc.CapabilityTags = append(doc.CapabilityTags, defaultCapabilityTag(doc))
	}
	if doc.InteractionMode == "" {
		if strings.EqualFold(firstNonBlank(doc.CardSupport), "streaming") || strings.EqualFold(firstNonBlank(doc.CardSupport), "both") {
			doc.InteractionMode = "interactive"
		} else {
			doc.InteractionMode = "stateless"
		}
	}
	if doc.CardSupport == "" {
		doc.CardSupport = inferCardSupport(doc)
	}
}

func validateStrictContract(meta map[string]any, doc *Document) []string {
	issues := make([]string, 0, 8)
	if issue := strictRequiredStringFieldIssue(meta, "name"); issue != "" {
		issues = append(issues, issue)
	}
	if issue := strictRequiredStringFieldIssue(meta, "version"); issue != "" {
		issues = append(issues, issue)
	}
	if issue := strictRequiredStringFieldIssue(meta, "description"); issue != "" {
		issues = append(issues, issue)
	}

	invocation, invocationOK, invocationIssue := strictRequiredStringField(meta, "invocation")
	if invocationIssue != "" {
		issues = append(issues, invocationIssue)
	} else if invocationOK && !isParseableBlueCommand(invocation) {
		issues = append(issues, "frontmatter invocation must be a parseable `blue ...` command")
	}

	examples, examplesOK, examplesIssue := strictRequiredStringListField(meta, "examples")
	if examplesIssue != "" {
		issues = append(issues, examplesIssue)
	} else if examplesOK {
		for _, example := range examples {
			if !isParseableBlueCommand(example) {
				issues = append(issues, "frontmatter examples must be parseable `blue ...` commands")
				break
			}
		}
	}

	if _, _, issue := strictRequiredStringListField(meta, "capability_tags"); issue != "" {
		issues = append(issues, issue)
	}
	if issue := strictRequiredStringFieldIssue(meta, "interaction_mode"); issue != "" {
		issues = append(issues, issue)
	}
	if issue := strictCardSupportFieldIssue(meta, "card_support"); issue != "" {
		issues = append(issues, issue)
	}
	return issues
}

func strictRequiredStringFieldIssue(meta map[string]any, key string) string {
	_, _, issue := strictRequiredStringField(meta, key)
	return issue
}

func strictRequiredStringField(meta map[string]any, key string) (string, bool, string) {
	raw, exists := meta[key]
	if !exists || raw == nil {
		return "", false, fmt.Sprintf("missing required frontmatter field: %s", key)
	}
	value, ok := raw.(string)
	if !ok {
		return "", false, fmt.Sprintf("frontmatter field %s must be a string", key)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, fmt.Sprintf("missing required frontmatter field: %s", key)
	}
	return value, true, ""
}

func strictRequiredStringListField(meta map[string]any, key string) ([]string, bool, string) {
	raw, exists := meta[key]
	if !exists || raw == nil {
		return nil, false, fmt.Sprintf("missing required frontmatter field: %s", key)
	}
	values, ok := strictStringListValue(raw)
	if !ok {
		return nil, false, fmt.Sprintf("frontmatter field %s must be a YAML list of strings", key)
	}
	if len(values) == 0 {
		return nil, false, fmt.Sprintf("missing required frontmatter field: %s", key)
	}
	return values, true, ""
}

func strictStringListValue(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return appendUnique(nil, v...), true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			value, ok := item.(string)
			if !ok {
				return nil, false
			}
			value = strings.TrimSpace(value)
			if value != "" {
				out = append(out, value)
			}
		}
		return appendUnique(nil, out...), true
	default:
		return nil, false
	}
}

func strictCardSupportFieldIssue(meta map[string]any, key string) string {
	value, ok, issue := strictRequiredStringField(meta, key)
	if issue != "" {
		return issue
	}
	if !ok {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none", "batch", "streaming", "both":
		return ""
	default:
		return fmt.Sprintf("frontmatter field %s must be one of none, batch, streaming, both", key)
	}
}

func defaultCapabilityTag(doc *Document) string {
	if doc == nil {
		return "general"
	}
	if strings.TrimSpace(doc.Category) != "" {
		return normalizeSkillID(doc.Category)
	}
	switch {
	case strings.Contains(strings.ToLower(doc.Description), "browser"), strings.Contains(strings.ToLower(doc.Invocation), "browser"):
		return "browser"
	case strings.Contains(strings.ToLower(doc.Description), "search"), strings.Contains(strings.ToLower(doc.Invocation), "search"):
		return "search"
	default:
		return "general"
	}
}

func inferCardSupport(doc *Document) string {
	if doc == nil {
		return "none"
	}
	lowerBody := strings.ToLower(doc.Body)
	switch {
	case strings.Contains(lowerBody, "__card__"), strings.Contains(lowerBody, "emitcard"), strings.Contains(lowerBody, "screenshot"), strings.Contains(lowerBody, "progress"):
		return "streaming"
	default:
		return "none"
	}
}

func normalizeCardSupport(raw any) string {
	switch v := raw.(type) {
	case bool:
		if v {
			return "batch"
		}
		return "none"
	case string:
		value := strings.ToLower(strings.TrimSpace(v))
		switch value {
		case "", "none":
			return value
		case "batch", "streaming", "both":
			return value
		default:
			return value
		}
	default:
		return ""
	}
}

func boolValue(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	}
	return false, false
}

func stringValue(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func stringListValue(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case []string:
		return appendUnique(nil, v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := stringValue(item); s != "" {
				out = append(out, s)
			}
		}
		return appendUnique(nil, out...)
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return nil
		}
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			value = strings.TrimSpace(value[1 : len(value)-1])
		}
		if strings.Contains(value, "\n") {
			lines := strings.Split(value, "\n")
			out := make([]string, 0, len(lines))
			for _, line := range lines {
				line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
				line = strings.Trim(line, `"'`)
				if line != "" {
					out = append(out, line)
				}
			}
			return appendUnique(nil, out...)
		}
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.Trim(strings.TrimSpace(part), `"'`)
			if part != "" {
				out = append(out, part)
			}
		}
		return appendUnique(nil, out...)
	default:
		return nil
	}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeSkillID(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, "/", "_")
	ensureNonSlugChars()
	value = nonSlugChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_.")
	if value == "" {
		return "skill"
	}
	return value
}

func isParseableBlueCommand(raw string) bool {
	fields := strings.Fields(strings.TrimSpace(raw))
	return len(fields) >= 2 && fields[0] == "blue"
}

func splitSections(body string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(body, "\n")
	currentTitle := ""
	var current []string
	flush := func() {
		if currentTitle == "" {
			return
		}
		joined := strings.TrimSpace(strings.Join(current, "\n"))
		if joined != "" {
			sections[currentTitle] = joined
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			currentTitle = normalizeHeading(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			current = current[:0]
			continue
		}
		if currentTitle != "" {
			current = append(current, line)
		}
	}
	flush()
	return sections
}

func normalizeHeading(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = strings.Trim(title, ":-_[]()")
	title = strings.Join(strings.Fields(title), " ")
	return title
}

func firstSectionContent(sections map[string]string, aliases ...string) string {
	for _, alias := range aliases {
		if value, ok := sections[normalizeHeading(alias)]; ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stripMarkdownTableRows(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractScriptPaths(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	ensureSkillScriptPathRegexp()
	matches := skillScriptPathRegexp.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		p := strings.TrimSpace(match[1])
		p = strings.TrimRight(p, `"'.,;:)]}`)
		if p != "" {
			out = append(out, p)
		}
	}
	return appendUnique(nil, out...)
}

func extractCommands(content string) []string {
	lines := strings.Split(content, "\n")
	inFence := false
	var commands []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "$ ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "$ "))
		}
		commands = append(commands, truncateForIndex(trimmed, 220))
	}
	return appendUnique(nil, commands...)
}

func parseTaskRoutes(section string) []Route {
	rows := parseMarkdownTable(section)
	if len(rows) < 2 {
		return nil
	}
	header := normalizeTableHeaderMap(rows[0])
	intentIdx := findTableColumn(header, "user intent", "intent", "用户意图", "意图", "需求")
	actionIdx := findTableColumn(header, "action", "command", "操作", "执行", "动作")
	if intentIdx < 0 || actionIdx < 0 {
		return nil
	}
	var out []Route
	for _, row := range rows[1:] {
		if intentIdx >= len(row) || actionIdx >= len(row) {
			continue
		}
		intent := strings.TrimSpace(row[intentIdx])
		action := strings.TrimSpace(row[actionIdx])
		if intent == "" || action == "" {
			continue
		}
		out = append(out, Route{Intent: intent, Action: action})
	}
	return out
}

func parseErrorRules(section string) []ErrorRule {
	rows := parseMarkdownTable(section)
	if len(rows) < 2 {
		return nil
	}
	header := normalizeTableHeaderMap(rows[0])
	errorIdx := findTableColumn(header, "error", "issue", "problem", "错误")
	resolutionIdx := findTableColumn(header, "resolution", "handling", "fix", "解决", "处理")
	if errorIdx < 0 || resolutionIdx < 0 {
		return nil
	}
	var out []ErrorRule
	for _, row := range rows[1:] {
		if errorIdx >= len(row) || resolutionIdx >= len(row) {
			continue
		}
		errText := strings.TrimSpace(row[errorIdx])
		resText := strings.TrimSpace(row[resolutionIdx])
		if errText == "" || resText == "" {
			continue
		}
		out = append(out, ErrorRule{Error: errText, Resolution: resText})
	}
	return out
}

func parseMarkdownTable(section string) [][]string {
	if strings.TrimSpace(section) == "" {
		return nil
	}
	lines := strings.Split(section, "\n")
	rows := make([][]string, 0, 8)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		row := splitMarkdownTableRow(trimmed)
		if len(row) == 0 || isMarkdownTableSeparator(row) {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|"))
	var (
		cells []string
		buf   strings.Builder
		esc   bool
	)
	for _, r := range line {
		if esc {
			buf.WriteRune(r)
			esc = false
			continue
		}
		if r == '\\' {
			esc = true
			continue
		}
		if r == '|' {
			cells = append(cells, strings.TrimSpace(buf.String()))
			buf.Reset()
			continue
		}
		buf.WriteRune(r)
	}
	cells = append(cells, strings.TrimSpace(buf.String()))
	return cells
}

func isMarkdownTableSeparator(row []string) bool {
	if len(row) == 0 {
		return false
	}
	for _, cell := range row {
		cell = strings.Trim(strings.TrimSpace(cell), ":")
		if cell == "" {
			continue
		}
		for _, r := range cell {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func normalizeTableHeaderMap(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, h := range header {
		out[normalizeHeading(h)] = i
	}
	return out
}

func findTableColumn(header map[string]int, aliases ...string) int {
	for _, alias := range aliases {
		if idx, ok := header[normalizeHeading(alias)]; ok {
			return idx
		}
	}
	return -1
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, value)
	}
	sort.Strings(dst)
	return dst
}

func extractFirstParagraph(content string) string {
	lines := strings.Split(content, "\n")
	pastHeading := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			pastHeading = true
			continue
		}
		if pastHeading || !strings.HasPrefix(trimmed, "---") {
			if len(trimmed) > 200 {
				return trimmed[:197] + "..."
			}
			return trimmed
		}
	}
	return ""
}

func extractSkillExample(body, skillName string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "blue ") || strings.HasPrefix(trimmed, skillName+" ") {
			return truncateForIndex(trimmed, 180)
		}
		if strings.Contains(trimmed, "`blue ") || strings.Contains(trimmed, "`"+skillName+" ") {
			trimmed = strings.Trim(trimmed, "`")
			return truncateForIndex(trimmed, 180)
		}
	}
	return ""
}

func truncateForIndex(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-3]) + "..."
}

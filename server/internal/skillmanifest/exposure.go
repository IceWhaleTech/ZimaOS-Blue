package skillmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type SkillActivationState string

const (
	SkillActivationStateActive  SkillActivationState = "active"
	SkillActivationStateDormant SkillActivationState = "dormant"
)

type SkillActivationSource string

const (
	SkillActivationSourceBaseline          SkillActivationSource = "baseline"
	SkillActivationSourceConditional       SkillActivationSource = "conditional"
	SkillActivationSourceDynamicDiscovered SkillActivationSource = "dynamic_discovered"
)

type SkillExposureView struct {
	Document         Document
	Raw              []byte
	Root             string
	EntryDir         string
	Embedded         bool
	Paths            []string
	UserInvocable    bool
	ModelInvocable   bool
	ActivationState  string
	ActivationSource string
}

type SkillExposureSnapshot struct {
	VisibleSkills              []SkillExposureView
	ActiveSkills               []SkillExposureView
	DormantSkills              []SkillExposureView
	ActiveSkillCount           int
	DormantSkillCount          int
	DiscoveredDirs             []string
	ActivatedConditionalSkills []string
	Stamp                      string
}

type SkillExposureManager struct {
	workspaceDir string

	mu                           sync.RWMutex
	dynamicExposureEnabled       func() bool
	touchedPaths                 map[string]struct{}
	discoveredRoots              map[string]discoveredSkillRoot
	discoveredDirs               []string
	activatedConditionalSkillSet map[string]struct{}
	activatedConditionalSkills   []string
}

type discoveredSkillRoot struct {
	Root         string
	Depth        int
	KindPriority int
}

type exposureRootCandidate struct {
	Root         string
	Embedded     bool
	Dynamic      bool
	Depth        int
	KindPriority int
	Priority     int
}

type exposureSlot struct {
	active  *SkillExposureView
	dormant *SkillExposureView
}

type skillExposureState struct {
	dynamicEnabled  bool
	touchedPaths    []string
	discoveredRoots map[string]discoveredSkillRoot
	discoveredDirs  []string
}

var (
	sharedSkillExposureManagers sync.Map
	pathGlobRegexpCache         sync.Map
	simpleGitignoreCache        sync.Map
)

func SharedSkillExposureManager(workspaceDir string) *SkillExposureManager {
	cleanWorkspace := cleanWorkspaceDir(workspaceDir)
	if cleanWorkspace == "" {
		return NewSkillExposureManager("")
	}
	if existing, ok := sharedSkillExposureManagers.Load(cleanWorkspace); ok {
		return existing.(*SkillExposureManager)
	}
	manager := NewSkillExposureManager(cleanWorkspace)
	actual, _ := sharedSkillExposureManagers.LoadOrStore(cleanWorkspace, manager)
	return actual.(*SkillExposureManager)
}

func NewSkillExposureManager(workspaceDir string) *SkillExposureManager {
	return &SkillExposureManager{
		workspaceDir:                 cleanWorkspaceDir(workspaceDir),
		touchedPaths:                 make(map[string]struct{}),
		discoveredRoots:              make(map[string]discoveredSkillRoot),
		activatedConditionalSkillSet: make(map[string]struct{}),
	}
}

func (m *SkillExposureManager) WorkspaceDir() string {
	if m == nil {
		return ""
	}
	return m.workspaceDir
}

func (m *SkillExposureManager) SetDynamicExposureEnabledFunc(fn func() bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dynamicExposureEnabled = fn
}

func (m *SkillExposureManager) DynamicExposureEnabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	fn := m.dynamicExposureEnabled
	m.mu.RUnlock()
	if fn == nil {
		return false
	}
	return fn()
}

func (m *SkillExposureManager) ObserveToolPath(toolName, absPath string) {
	if m == nil || !m.DynamicExposureEnabled() || !isSkillExposureTriggerTool(toolName) {
		return
	}

	workspaceDir := cleanWorkspaceDir(m.workspaceDir)
	if workspaceDir == "" {
		return
	}

	absPath = strings.TrimSpace(absPath)
	if absPath == "" {
		return
	}
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(workspaceDir, absPath)
	}
	absPath = filepath.Clean(absPath)

	relPath, ok := exposureRelPath(workspaceDir, absPath)
	if !ok {
		return
	}

	newRoots := m.discoverNestedRoots(absPath)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.touchedPaths[relPath]; !exists {
		m.touchedPaths[relPath] = struct{}{}
	}
	for _, root := range newRoots {
		if _, exists := m.discoveredRoots[root.Root]; exists {
			continue
		}
		m.discoveredRoots[root.Root] = root
		m.discoveredDirs = append(m.discoveredDirs, root.Root)
	}
}

func (m *SkillExposureManager) Snapshot() (SkillExposureSnapshot, error) {
	if m == nil {
		return SkillExposureSnapshot{}, nil
	}
	if strings.TrimSpace(m.workspaceDir) == "" {
		return SkillExposureSnapshot{}, nil
	}
	if err := ValidateCanonicalConflicts(m.workspaceDir, Options{}, true); err != nil {
		return SkillExposureSnapshot{}, err
	}

	state := m.cloneState()
	candidates, err := m.collectCandidates(state)
	if err != nil {
		return SkillExposureSnapshot{}, err
	}

	snapshot := buildExposureSnapshotFromCandidates(candidates)
	snapshot.DiscoveredDirs = append([]string(nil), state.discoveredDirs...)
	m.recordActivatedConditionalSkills(&snapshot)
	snapshot.ActiveSkillCount = len(snapshot.ActiveSkills)
	snapshot.DormantSkillCount = len(snapshot.DormantSkills)
	snapshot.Stamp = buildExposureStamp(snapshot.VisibleSkills)
	return snapshot, nil
}

func (m *SkillExposureManager) cloneState() skillExposureState {
	if m == nil {
		return skillExposureState{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	touched := make([]string, 0, len(m.touchedPaths))
	for relPath := range m.touchedPaths {
		touched = append(touched, relPath)
	}
	sort.Strings(touched)

	discoveredRoots := make(map[string]discoveredSkillRoot, len(m.discoveredRoots))
	for root, info := range m.discoveredRoots {
		discoveredRoots[root] = info
	}

	return skillExposureState{
		dynamicEnabled:  m.dynamicExposureEnabled != nil && m.dynamicExposureEnabled(),
		touchedPaths:    touched,
		discoveredRoots: discoveredRoots,
		discoveredDirs:  append([]string(nil), m.discoveredDirs...),
	}
}

func (m *SkillExposureManager) recordActivatedConditionalSkills(snapshot *SkillExposureSnapshot) {
	if m == nil || snapshot == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, view := range snapshot.ActiveSkills {
		if view.ActivationSource != string(SkillActivationSourceConditional) {
			continue
		}
		key := skillExposureViewKey(view)
		if key == "" {
			continue
		}
		if _, exists := m.activatedConditionalSkillSet[key]; exists {
			continue
		}
		m.activatedConditionalSkillSet[key] = struct{}{}
		m.activatedConditionalSkills = append(m.activatedConditionalSkills, key)
	}

	snapshot.ActivatedConditionalSkills = append([]string(nil), m.activatedConditionalSkills...)
}

func (m *SkillExposureManager) collectCandidates(state skillExposureState) ([]SkillExposureView, error) {
	roots := buildExposureRootCandidates(m.workspaceDir, state)
	candidates := make([]SkillExposureView, 0, 64)

	for _, root := range roots {
		if root.Embedded {
			embeddedViews, err := collectEmbeddedExposureViews(state, root)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, embeddedViews...)
			continue
		}
		rootViews, err := collectRootExposureViews(root, state)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, rootViews...)
	}

	return candidates, nil
}

func buildExposureRootCandidates(workspaceDir string, state skillExposureState) []exposureRootCandidate {
	staticRoots := ResolveRoots(workspaceDir)
	staticRootSet := make(map[string]struct{}, len(staticRoots))
	for _, root := range staticRoots {
		staticRootSet[filepath.Clean(root)] = struct{}{}
	}

	candidates := make([]exposureRootCandidate, 0, len(staticRoots)+len(state.discoveredRoots)+1)
	if state.dynamicEnabled {
		discovered := make([]discoveredSkillRoot, 0, len(state.discoveredRoots))
		for _, info := range state.discoveredRoots {
			if _, exists := staticRootSet[filepath.Clean(info.Root)]; exists {
				continue
			}
			discovered = append(discovered, info)
		}
		sort.Slice(discovered, func(i, j int) bool {
			if discovered[i].Depth != discovered[j].Depth {
				return discovered[i].Depth > discovered[j].Depth
			}
			if discovered[i].KindPriority != discovered[j].KindPriority {
				return discovered[i].KindPriority < discovered[j].KindPriority
			}
			return discovered[i].Root < discovered[j].Root
		})
		for idx, info := range discovered {
			candidates = append(candidates, exposureRootCandidate{
				Root:         info.Root,
				Dynamic:      true,
				Depth:        info.Depth,
				KindPriority: info.KindPriority,
				Priority:     idx,
			})
		}
	}

	for idx, root := range staticRoots {
		candidates = append(candidates, exposureRootCandidate{
			Root:     root,
			Priority: len(candidates) + idx,
		})
	}

	candidates = append(candidates, exposureRootCandidate{
		Embedded: true,
		Priority: len(candidates),
	})
	return candidates
}

func collectRootExposureViews(root exposureRootCandidate, state skillExposureState) ([]SkillExposureView, error) {
	entries, err := os.ReadDir(root.Root)
	if err != nil {
		return nil, nil
	}

	views := make([]SkillExposureView, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		entryDir := filepath.Join(root.Root, entry.Name())
		doc, raw, err := ReadDir(entryDir, Options{})
		if err != nil {
			continue
		}
		if !doc.Enabled || !PlatformMatch(doc.OS) {
			continue
		}
		stateValue, sourceValue := resolveSkillActivation(doc, root.Dynamic, state)
		views = append(views, SkillExposureView{
			Document:         doc,
			Raw:              append([]byte(nil), raw...),
			Root:             root.Root,
			EntryDir:         entryDir,
			Embedded:         false,
			Paths:            append([]string(nil), doc.Paths...),
			UserInvocable:    doc.UserInvocable,
			ModelInvocable:   doc.ModelInvocable,
			ActivationState:  string(stateValue),
			ActivationSource: string(sourceValue),
		})
	}
	return views, nil
}

func collectEmbeddedExposureViews(state skillExposureState, root exposureRootCandidate) ([]SkillExposureView, error) {
	embeddedIDs, err := ListEmbeddedIDs()
	if err != nil {
		return nil, err
	}

	views := make([]SkillExposureView, 0, len(embeddedIDs))
	for _, skillID := range embeddedIDs {
		doc, raw, err := ReadEmbedded(skillID, Options{})
		if err != nil {
			continue
		}
		if !doc.Enabled || !PlatformMatch(doc.OS) {
			continue
		}
		stateValue, sourceValue := resolveSkillActivation(doc, root.Dynamic, state)
		views = append(views, SkillExposureView{
			Document:         doc,
			Raw:              append([]byte(nil), raw...),
			Root:             root.Root,
			EntryDir:         skillID,
			Embedded:         true,
			Paths:            append([]string(nil), doc.Paths...),
			UserInvocable:    doc.UserInvocable,
			ModelInvocable:   doc.ModelInvocable,
			ActivationState:  string(stateValue),
			ActivationSource: string(sourceValue),
		})
	}
	return views, nil
}

func buildExposureSnapshotFromCandidates(candidates []SkillExposureView) SkillExposureSnapshot {
	if len(candidates) == 0 {
		return SkillExposureSnapshot{}
	}

	slots := make([]exposureSlot, 0, len(candidates))
	aliasToSlot := make(map[string]int, len(candidates)*2)

	for _, candidate := range candidates {
		slotIndex := findExposureSlotIndex(candidate, aliasToSlot)
		if slotIndex == -1 {
			slotIndex = len(slots)
			slots = append(slots, exposureSlot{})
		}
		slot := &slots[slotIndex]
		if candidate.ActivationState == string(SkillActivationStateActive) {
			if slot.active == nil {
				view := candidate
				slot.active = &view
			}
		} else if slot.dormant == nil {
			view := candidate
			slot.dormant = &view
		}
		registerExposureAliases(slotIndex, exposureAliases(candidate), aliasToSlot)
	}

	snapshot := SkillExposureSnapshot{
		VisibleSkills: make([]SkillExposureView, 0, len(slots)),
		ActiveSkills:  make([]SkillExposureView, 0, len(slots)),
		DormantSkills: make([]SkillExposureView, 0, len(slots)),
	}
	for _, slot := range slots {
		switch {
		case slot.active != nil:
			snapshot.VisibleSkills = append(snapshot.VisibleSkills, *slot.active)
			snapshot.ActiveSkills = append(snapshot.ActiveSkills, *slot.active)
		case slot.dormant != nil:
			snapshot.VisibleSkills = append(snapshot.VisibleSkills, *slot.dormant)
			snapshot.DormantSkills = append(snapshot.DormantSkills, *slot.dormant)
		}
	}

	sortExposureViews(snapshot.VisibleSkills)
	sortExposureViews(snapshot.ActiveSkills)
	sortExposureViews(snapshot.DormantSkills)
	return snapshot
}

func resolveSkillActivation(doc Document, dynamicRoot bool, state skillExposureState) (SkillActivationState, SkillActivationSource) {
	if !state.dynamicEnabled {
		if dynamicRoot {
			return SkillActivationStateActive, SkillActivationSourceDynamicDiscovered
		}
		return SkillActivationStateActive, SkillActivationSourceBaseline
	}

	patterns, conditional := normalizeExposurePaths(doc.Paths)
	if conditional {
		for _, touched := range state.touchedPaths {
			if exposurePatternsMatch(patterns, touched) {
				return SkillActivationStateActive, SkillActivationSourceConditional
			}
		}
		return SkillActivationStateDormant, SkillActivationSourceConditional
	}
	if dynamicRoot {
		return SkillActivationStateActive, SkillActivationSourceDynamicDiscovered
	}
	return SkillActivationStateActive, SkillActivationSourceBaseline
}

func normalizeExposurePaths(rawPaths []string) ([]string, bool) {
	patterns := make([]string, 0, len(rawPaths))
	sawNonBlank := false
	for _, raw := range rawPaths {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		sawNonBlank = true
		pattern := normalizeExposurePathPattern(raw)
		switch pattern {
		case "":
			continue
		case "**":
			return nil, false
		default:
			patterns = append(patterns, pattern)
		}
	}
	if len(patterns) == 0 {
		return nil, sawNonBlank
	}
	return append([]string(nil), patterns...), true
}

func normalizeExposurePathPattern(raw string) string {
	pattern := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if pattern == "" {
		return ""
	}
	for strings.HasPrefix(pattern, "./") {
		pattern = strings.TrimPrefix(pattern, "./")
	}
	for strings.Contains(pattern, "//") {
		pattern = strings.ReplaceAll(pattern, "//", "/")
	}
	if pattern == "**" {
		return pattern
	}
	if strings.HasSuffix(pattern, "/**") {
		pattern = strings.TrimSuffix(pattern, "/**")
	}
	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	if pattern == "" {
		return ""
	}
	if pattern == ".." || strings.HasPrefix(pattern, "../") || strings.Contains(pattern, "/../") {
		return ""
	}
	return pattern
}

func exposurePatternsMatch(patterns []string, relPath string) bool {
	relPath = strings.TrimSpace(strings.ReplaceAll(relPath, "\\", "/"))
	if relPath == "" {
		return false
	}
	for _, pattern := range patterns {
		if exposurePatternMatches(pattern, relPath) {
			return true
		}
	}
	return false
}

func exposurePatternMatches(pattern, relPath string) bool {
	if pattern == "" || relPath == "" {
		return false
	}
	if !strings.Contains(pattern, "/") {
		if !strings.ContainsAny(pattern, "*?[") {
			return relPath == pattern || strings.HasPrefix(relPath, pattern+"/")
		}
		return exposureGlobMatch(pattern, pathBase(relPath))
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return relPath == pattern || strings.HasPrefix(relPath, pattern+"/")
	}
	return exposureGlobMatch(pattern, relPath)
}

func exposureGlobMatch(pattern, candidate string) bool {
	pattern = strings.TrimSpace(pattern)
	candidate = strings.TrimSpace(candidate)
	if pattern == "" || candidate == "" {
		return false
	}
	regexAny, ok := pathGlobRegexpCache.Load(pattern)
	if ok {
		return regexAny.(*regexp.Regexp).MatchString(candidate)
	}

	var builder strings.Builder
	builder.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				builder.WriteString(".*")
				i++
			} else {
				builder.WriteString("[^/]*")
			}
		case '?':
			builder.WriteString("[^/]")
		default:
			if strings.ContainsRune(`.+^$(){}|[]\`, rune(pattern[i])) {
				builder.WriteByte('\\')
			}
			builder.WriteByte(pattern[i])
		}
	}
	builder.WriteString("$")

	compiled, err := regexp.Compile(builder.String())
	if err != nil {
		return false
	}
	actual, _ := pathGlobRegexpCache.LoadOrStore(pattern, compiled)
	return actual.(*regexp.Regexp).MatchString(candidate)
}

func (m *SkillExposureManager) discoverNestedRoots(absPath string) []discoveredSkillRoot {
	workspaceDir := cleanWorkspaceDir(m.workspaceDir)
	if workspaceDir == "" {
		return nil
	}

	startDir := filepath.Dir(absPath)
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		startDir = absPath
	}
	startDir = filepath.Clean(startDir)
	if !pathWithinExposureRoot(workspaceDir, startDir) {
		return nil
	}

	staticRoots := ResolveRoots(workspaceDir)
	staticRootSet := make(map[string]struct{}, len(staticRoots))
	for _, root := range staticRoots {
		staticRootSet[filepath.Clean(root)] = struct{}{}
	}

	discovered := make([]discoveredSkillRoot, 0, 4)
	for current := startDir; current != workspaceDir && pathWithinExposureRoot(workspaceDir, current); current = filepath.Dir(current) {
		scopeRoot := filepath.Clean(current)
		candidates := []discoveredSkillRoot{
			{
				Root:         filepath.Join(scopeRoot, ".agents", "skills"),
				Depth:        exposureRootDepth(workspaceDir, scopeRoot),
				KindPriority: 0,
			},
			{
				Root:         filepath.Join(scopeRoot, ".claude", "skills"),
				Depth:        exposureRootDepth(workspaceDir, scopeRoot),
				KindPriority: 1,
			},
		}
		for _, candidate := range candidates {
			candidateRoot := filepath.Clean(candidate.Root)
			if _, exists := staticRootSet[candidateRoot]; exists {
				continue
			}
			if !pathWithinExposureRoot(workspaceDir, candidateRoot) {
				continue
			}
			if !exposureDirExists(candidateRoot) {
				continue
			}
			if exposurePathGitIgnored(workspaceDir, candidateRoot) {
				continue
			}
			discovered = append(discovered, discoveredSkillRoot{
				Root:         candidateRoot,
				Depth:        candidate.Depth,
				KindPriority: candidate.KindPriority,
			})
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	sort.Slice(discovered, func(i, j int) bool {
		if discovered[i].Depth != discovered[j].Depth {
			return discovered[i].Depth > discovered[j].Depth
		}
		if discovered[i].KindPriority != discovered[j].KindPriority {
			return discovered[i].KindPriority < discovered[j].KindPriority
		}
		return discovered[i].Root < discovered[j].Root
	})
	return discovered
}

func findExposureSlotIndex(view SkillExposureView, aliasToSlot map[string]int) int {
	for _, alias := range exposureAliases(view) {
		if slotIndex, ok := aliasToSlot[alias]; ok {
			return slotIndex
		}
	}
	return -1
}

func registerExposureAliases(slotIndex int, aliases []string, aliasToSlot map[string]int) {
	for _, alias := range aliases {
		if alias == "" {
			continue
		}
		if _, exists := aliasToSlot[alias]; exists {
			continue
		}
		aliasToSlot[alias] = slotIndex
	}
}

func exposureAliases(view SkillExposureView) []string {
	aliases := make([]string, 0, 4)
	aliases = append(aliases, exposureCanonicalAliases(view)...)
	if base := strings.TrimSpace(filepath.Base(view.EntryDir)); base != "" && base != "." {
		aliases = append(aliases, strings.ToLower(base))
	}
	return uniqueLowerStrings(aliases)
}

func exposureCanonicalAliases(view SkillExposureView) []string {
	out := make([]string, 0, 2)
	if name := strings.TrimSpace(view.Document.Name); name != "" {
		out = append(out, strings.ToLower(name))
	}
	if id := strings.TrimSpace(view.Document.ID); id != "" {
		out = append(out, strings.ToLower(id))
	}
	return uniqueLowerStrings(out)
}

func sortExposureViews(views []SkillExposureView) {
	sort.Slice(views, func(i, j int) bool {
		nameI := strings.ToLower(strings.TrimSpace(firstNonBlank(views[i].Document.Name, views[i].Document.ID)))
		nameJ := strings.ToLower(strings.TrimSpace(firstNonBlank(views[j].Document.Name, views[j].Document.ID)))
		if nameI != nameJ {
			return nameI < nameJ
		}
		return skillExposureViewKey(views[i]) < skillExposureViewKey(views[j])
	})
}

func buildExposureStamp(views []SkillExposureView) string {
	if len(views) == 0 {
		return ""
	}
	parts := make([]string, 0, len(views))
	for _, view := range views {
		rawHash := sha256.Sum256(view.Raw)
		parts = append(parts,
			skillExposureViewKey(view)+"|"+
				strings.TrimSpace(view.Document.Location)+"|"+
				hex.EncodeToString(rawHash[:])+"|"+
				strings.TrimSpace(view.ActivationState)+"|"+
				strings.TrimSpace(view.ActivationSource),
		)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func skillExposureViewKey(view SkillExposureView) string {
	if id := strings.TrimSpace(view.Document.ID); id != "" {
		return id
	}
	if name := strings.TrimSpace(view.Document.Name); name != "" {
		return name
	}
	if base := strings.TrimSpace(filepath.Base(view.EntryDir)); base != "" && base != "." {
		return base
	}
	return strings.TrimSpace(view.Document.Location)
}

func cleanWorkspaceDir(workspaceDir string) string {
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return ""
	}
	if !filepath.IsAbs(workspaceDir) {
		if abs, err := filepath.Abs(workspaceDir); err == nil {
			workspaceDir = abs
		}
	}
	return filepath.Clean(workspaceDir)
}

func exposureRelPath(workspaceDir, absPath string) (string, bool) {
	workspaceDir = cleanWorkspaceDir(workspaceDir)
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if workspaceDir == "" || absPath == "" || !pathWithinExposureRoot(workspaceDir, absPath) {
		return "", false
	}
	rel, err := filepath.Rel(workspaceDir, absPath)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

func pathWithinExposureRoot(root, target string) bool {
	root = cleanWorkspaceDir(root)
	target = filepath.Clean(strings.TrimSpace(target))
	if root == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func exposureRootDepth(workspaceDir, candidate string) int {
	rel, ok := exposureRelPath(workspaceDir, candidate)
	if !ok {
		return 0
	}
	return len(strings.Split(rel, "/"))
}

func exposureDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isSkillExposureTriggerTool(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "read", "write", "edit", "file_read", "file_write":
		return true
	default:
		return false
	}
}

func pathBase(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}

func uniqueLowerStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

type gitignorePattern struct {
	baseDir  string
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool
}

func exposurePathGitIgnored(workspaceDir, absPath string) bool {
	workspaceDir = cleanWorkspaceDir(workspaceDir)
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if workspaceDir == "" || absPath == "" || !pathWithinExposureRoot(workspaceDir, absPath) {
		return false
	}

	dirs := exposureGitignoreSearchDirs(workspaceDir, absPath)
	patterns := make([]gitignorePattern, 0, len(dirs))
	for _, dir := range dirs {
		patterns = append(patterns, loadGitignorePatterns(dir)...)
	}
	ignored := false
	for _, pattern := range patterns {
		if gitignorePatternMatches(pattern, absPath) {
			ignored = !pattern.negate
		}
	}
	return ignored
}

func exposureGitignoreSearchDirs(workspaceDir, absPath string) []string {
	workspaceDir = cleanWorkspaceDir(workspaceDir)
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if workspaceDir == "" || absPath == "" {
		return nil
	}
	dir := absPath
	if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
		dir = filepath.Dir(absPath)
	}
	dirs := make([]string, 0, 8)
	for current := workspaceDir; pathWithinExposureRoot(current, dir); {
		dirs = append(dirs, current)
		if current == dir {
			break
		}
		nextRel, err := filepath.Rel(current, dir)
		if err != nil || nextRel == "." {
			break
		}
		segment := strings.Split(nextRel, string(os.PathSeparator))[0]
		current = filepath.Join(current, segment)
	}
	return dirs
}

func loadGitignorePatterns(dir string) []gitignorePattern {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" {
		return nil
	}
	if cached, ok := simpleGitignoreCache.Load(dir); ok {
		return cached.([]gitignorePattern)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		simpleGitignoreCache.Store(dir, []gitignorePattern(nil))
		return nil
	}

	lines := strings.Split(string(data), "\n")
	patterns := make([]gitignorePattern, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern := gitignorePattern{baseDir: dir}
		if strings.HasPrefix(line, "!") {
			pattern.negate = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
		}
		if strings.HasPrefix(line, "/") {
			pattern.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if strings.HasSuffix(line, "/") {
			pattern.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		line = normalizeExposurePathPattern(line)
		if line == "" {
			continue
		}
		pattern.pattern = line
		patterns = append(patterns, pattern)
	}

	simpleGitignoreCache.Store(dir, patterns)
	return patterns
}

func gitignorePatternMatches(pattern gitignorePattern, absPath string) bool {
	rel, err := filepath.Rel(pattern.baseDir, absPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}

	target := rel
	if !pattern.anchored && !strings.Contains(pattern.pattern, "/") {
		parts := strings.Split(rel, "/")
		for _, part := range parts {
			if gitignoreSingleMatch(pattern.pattern, part, pattern.dirOnly) {
				return true
			}
		}
		return false
	}
	return gitignoreSingleMatch(pattern.pattern, target, pattern.dirOnly)
}

func gitignoreSingleMatch(pattern, target string, dirOnly bool) bool {
	if pattern == "" || target == "" {
		return false
	}
	if !strings.ContainsAny(pattern, "*?[") {
		if dirOnly {
			return target == pattern || strings.HasPrefix(target, pattern+"/")
		}
		return target == pattern
	}
	if dirOnly {
		return exposurePatternMatches(pattern, target) || strings.HasPrefix(target, pattern+"/")
	}
	return exposurePatternMatches(pattern, target)
}

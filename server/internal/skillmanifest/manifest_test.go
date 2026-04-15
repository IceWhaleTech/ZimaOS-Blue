package skillmanifest

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
)

func writeCanonicalConflictSkill(t *testing.T, dir, entryDir, manifestName, description string) string {
	t.Helper()

	skillDir := filepath.Join(dir, entryDir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := `---
name: ` + manifestName + `
version: 1.0.0
description: ` + description + `
invocation: blue ` + manifestName + `
examples:
  - blue ` + manifestName + `
capability_tags:
  - test
interaction_mode: stateless
card_support: none
---
# ` + manifestName + `
`
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return path
}

func TestParseEntry_StrictContractRequiresFrontmatterFields(t *testing.T) {
	content := `---
name: reminder
description: Schedule reminders
---
# Reminder

## Command Usage

` + "```bash" + `
blue reminder add message="ping" time=10m
` + "```" + `
`

	_, err := ParseEntry("reminder", "/tmp/SKILL.md", []byte(content), Options{RequireContract: true})
	if err == nil {
		t.Fatal("expected strict contract validation error")
	}
	if !strings.Contains(err.Error(), "missing required frontmatter field: version") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestParseEntry_StrictContractRejectsFrontmatterTypeErrors(t *testing.T) {
	content := `---
name:
  nested: reminder
version: 7
description: true
invocation:
  - blue reminder add
examples: blue reminder add
capability_tags: reminder
interaction_mode:
  mode: stateless
card_support: realtime
---
# Reminder
`

	_, err := ParseEntry("reminder", "/tmp/SKILL.md", []byte(content), Options{RequireContract: true})
	if err == nil {
		t.Fatal("expected strict contract validation error")
	}
	for _, want := range []string{
		"frontmatter field name must be a string",
		"frontmatter field version must be a string",
		"frontmatter field description must be a string",
		"frontmatter field invocation must be a string",
		"frontmatter field examples must be a YAML list of strings",
		"frontmatter field capability_tags must be a YAML list of strings",
		"frontmatter field interaction_mode must be a string",
		"frontmatter field card_support must be one of none, batch, streaming, both",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error %q missing %q", err, want)
		}
	}
}

func TestParseEntry_StrictContractRejectsUnparseableBlueCommands(t *testing.T) {
	content := `---
name: reminder
version: 1.0.0
description: Schedule reminders
invocation: reminder add message=ping
examples:
  - reminder add message=ping
capability_tags:
  - reminder
interaction_mode: stateless
card_support: batch
---
# Reminder
`

	_, err := ParseEntry("reminder", "/tmp/SKILL.md", []byte(content), Options{RequireContract: true})
	if err == nil {
		t.Fatal("expected strict contract validation error")
	}
	if !strings.Contains(err.Error(), "frontmatter invocation must be a parseable `blue ...` command") {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if !strings.Contains(err.Error(), "frontmatter examples must be parseable `blue ...` commands") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestParseEntry_StrictContractAllowsLegacyFallback(t *testing.T) {
	content := `---
name: legacy_search
description: Search the web
---
# Legacy Search

## Command Usage

` + "```bash" + `
blue legacy_search query="latest docs"
` + "```" + `
`

	doc, err := ParseEntry("legacy_search", "/tmp/SKILL.md", []byte(content), Options{
		RequireContract:     true,
		AllowLegacyFallback: true,
	})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if doc.Invocation != `blue legacy_search query="latest docs"` {
		t.Fatalf("invocation = %q", doc.Invocation)
	}
	if len(doc.ValidationNotes) == 0 {
		t.Fatal("expected validation notes for legacy fallback")
	}
	if doc.Manifest == nil || doc.Manifest.Metadata["validation_notes"] == "" {
		t.Fatalf("expected manifest validation note metadata, got %+v", doc.Manifest)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractStatus]; got != ContractStatusLegacyFallback {
		t.Fatalf("contract_status = %q, want %q", got, ContractStatusLegacyFallback)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractSource]; got != ContractSourceLegacyFrontmatter {
		t.Fatalf("contract_source = %q, want %q", got, ContractSourceLegacyFrontmatter)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractNotes]; !strings.Contains(got, "legacy manifest compatibility fallback applied") {
		t.Fatalf("contract_notes = %q, want legacy fallback note", got)
	}
}

func TestParseEntry_StrictContractTracksDeclaredFrontmatterContract(t *testing.T) {
	content := `---
name: reminder
version: 1.0.0
description: Schedule reminders
invocation: blue reminder add message=ping
examples:
  - blue reminder add message=ping
capability_tags:
  - reminder
interaction_mode: stateless
card_support: batch
---
# Reminder
`

	doc, err := ParseEntry("reminder", "/tmp/SKILL.md", []byte(content), Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected manifest")
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractStatus]; got != ContractStatusStrict {
		t.Fatalf("contract_status = %q, want %q", got, ContractStatusStrict)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractSource]; got != ContractSourceDeclaredFrontmatter {
		t.Fatalf("contract_source = %q, want %q", got, ContractSourceDeclaredFrontmatter)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractNotes]; got != "" {
		t.Fatalf("contract_notes = %q, want empty", got)
	}
}

func TestParseEntry_StrictContractLegacyFallbackStillRejectsNoFrontmatter(t *testing.T) {
	content := "# Just a skill\n\nNo frontmatter here."

	_, err := ParseEntry("skill", "/tmp/SKILL.md", []byte(content), Options{
		RequireContract:     true,
		AllowLegacyFallback: true,
	})
	if err == nil {
		t.Fatal("expected strict contract validation error")
	}
	if !strings.Contains(err.Error(), "missing required frontmatter field: name") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestParseEntry_StrictContractLegacyFallbackRepairsInvalidContractFields(t *testing.T) {
	content := `---
name: legacy_reminder
description: Legacy reminder skill
invocation: reminder add message=ping
examples: reminder add message=ping
card_support: realtime
---
# Legacy Reminder

## Command Usage

` + "```bash" + `
blue legacy_reminder action=list
` + "```" + `
`

	doc, err := ParseEntry("legacy_reminder", "/tmp/SKILL.md", []byte(content), Options{
		RequireContract:     true,
		AllowLegacyFallback: true,
	})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if doc.Invocation != `blue legacy_reminder action=list` {
		t.Fatalf("invocation = %q", doc.Invocation)
	}
	if len(doc.Examples) == 0 || doc.Examples[0] != doc.Invocation {
		t.Fatalf("examples = %v", doc.Examples)
	}
	if doc.CardSupport != "none" {
		t.Fatalf("card_support = %q, want none", doc.CardSupport)
	}
}

func TestParseEntry_PermissiveModeDerivesLegacyFields(t *testing.T) {
	content := `---
name: web_search
description: Search the web
os: ["` + runtime.GOOS + `"]
---
# Web Search

## Command Usage

` + "```bash" + `
blue web_search query="latest news"
` + "```" + `
`

	doc, err := ParseEntry("web_search", "/tmp/SKILL.md", []byte(content), Options{})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if doc.Invocation != `blue web_search query="latest news"` {
		t.Fatalf("invocation = %q", doc.Invocation)
	}
	if len(doc.Examples) == 0 || doc.Examples[0] != doc.Invocation {
		t.Fatalf("examples = %v", doc.Examples)
	}
	if doc.InteractionMode == "" || doc.CardSupport == "" {
		t.Fatalf("expected derived interaction/card fields, got mode=%q card=%q", doc.InteractionMode, doc.CardSupport)
	}
	if doc.Manifest == nil || doc.Manifest.Invocation == "" {
		t.Fatalf("expected manifest invocation, got %+v", doc.Manifest)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractStatus]; got != ContractStatusLegacyFallback {
		t.Fatalf("contract_status = %q, want %q", got, ContractStatusLegacyFallback)
	}
}

func TestParseEntry_PermissiveModeMarksPlainMarkdownAsGeneratedContract(t *testing.T) {
	content := `
# Plain Third Party Skill

Installs without frontmatter.

blue plain-thirdparty action=list
`

	doc, err := ParseEntry("plain-thirdparty", "/tmp/SKILL.md", []byte(content), Options{})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected manifest")
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractStatus]; got != ContractStatusGenerated {
		t.Fatalf("contract_status = %q, want %q", got, ContractStatusGenerated)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractSource]; got != ContractSourceGeneratedSafeDefaults {
		t.Fatalf("contract_source = %q, want %q", got, ContractSourceGeneratedSafeDefaults)
	}
	if got := doc.Manifest.Metadata[ManifestMetadataContractNotes]; !strings.Contains(got, "generated safe structural contract defaults") {
		t.Fatalf("contract_notes = %q, want generated contract note", got)
	}
}

func TestParseEntry_ExposureFieldDefaults(t *testing.T) {
	content := `---
name: web_search
description: Search the web
os: ["` + runtime.GOOS + `"]
---
# Web Search
`

	doc, err := ParseEntry("web_search", "/tmp/SKILL.md", []byte(content), Options{})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if !doc.UserInvocable {
		t.Fatal("expected user_invocable to default to true")
	}
	if !doc.ModelInvocable {
		t.Fatal("expected model_invocable to default to true")
	}
	if len(doc.Paths) != 0 {
		t.Fatalf("expected no paths by default, got %v", doc.Paths)
	}
	if doc.Manifest == nil || !doc.Manifest.UserInvocable || !doc.Manifest.ModelInvocable {
		t.Fatalf("unexpected manifest defaults: %+v", doc.Manifest)
	}
}

func TestParseEntry_ExposureFieldAliases(t *testing.T) {
	content := `---
name: hidden_skill
version: "1.0.0"
description: Hidden internal skill
paths:
  - src/**
user-invocable: false
disable-model-invocation: true
invocation: "blue hidden_skill action=run"
examples:
  - "blue hidden_skill action=run"
capability_tags:
  - internal
interaction_mode: stateless
card_support: none
---
# Hidden Skill
`

	doc, err := ParseEntry("hidden_skill", "/tmp/SKILL.md", []byte(content), Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ParseEntry error: %v", err)
	}
	if got := doc.Paths; len(got) != 1 || got[0] != "src/**" {
		t.Fatalf("paths=%v, want [src/**]", got)
	}
	if doc.UserInvocable {
		t.Fatal("expected user_invocable=false from alias")
	}
	if doc.ModelInvocable {
		t.Fatal("expected model_invocable=false from disable-model-invocation")
	}
	if doc.Manifest == nil || doc.Manifest.UserInvocable || doc.Manifest.ModelInvocable {
		t.Fatalf("unexpected manifest alias values: %+v", doc.Manifest)
	}
}

func TestValidateArchiveInstallRoot_StrictContract(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "bundle.skill")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zipWriter := zip.NewWriter(file)
	skillFile, err := zipWriter.Create("bundle/SKILL.md")
	if err != nil {
		t.Fatalf("create skill entry: %v", err)
	}
	content := `---
name: archive_skill
version: 1.2.3
description: Install from .skill archive
invocation: blue archive_skill action=list
examples:
  - blue archive_skill action=list
capability_tags:
  - archive
interaction_mode: stateless
card_support: none
---
# Archive Skill
`
	if _, err := skillFile.Write([]byte(content)); err != nil {
		t.Fatalf("write skill entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatalf("mkdir extract: %v", err)
	}
	if err := skillbundle.ExtractArchiveFile(archivePath, extractDir, archivePath, "application/zip"); err != nil {
		t.Fatalf("extract archive: %v", err)
	}

	bundle, err := ValidateArchiveInstallRoot(extractDir, "archive_skill", Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ValidateArchiveInstallRoot error: %v", err)
	}
	if bundle.Document.ID != "archive_skill" {
		t.Fatalf("bundle.Document.ID = %q", bundle.Document.ID)
	}
	if bundle.Document.Manifest == nil || bundle.Document.Manifest.Version != "1.2.3" {
		t.Fatalf("unexpected manifest: %+v", bundle.Document.Manifest)
	}
}

func TestPinnedBuiltinSkills_StrictContractAndEmbeddedSync(t *testing.T) {
	pinned := []string{
		"ask",
		"browser",
		"web_query",
		"deep_research",
		"analyze",
		"ui_reviewer",
		"config",
		"mediagen",
		"reminder",
		"scheduler",
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	for _, id := range pinned {
		t.Run(id, func(t *testing.T) {
			assetPath := filepath.Join(repoRoot, "assets", "skills", id, "SKILL.md")
			assetData, err := os.ReadFile(assetPath)
			if err != nil {
				t.Fatalf("read asset skill: %v", err)
			}

			assetDoc, err := ParseEntry(id, assetPath, assetData, Options{RequireContract: true})
			if err != nil {
				t.Fatalf("ParseEntry asset strict contract error: %v", err)
			}
			if assetDoc.Manifest == nil {
				t.Fatal("expected asset manifest")
			}

			embeddedDoc, embeddedData, err := ReadEmbedded(id, Options{RequireContract: true})
			if err != nil {
				t.Fatalf("ReadEmbedded strict contract error: %v", err)
			}
			if embeddedDoc.Manifest == nil {
				t.Fatal("expected embedded manifest")
			}

			if string(assetData) != string(embeddedData) {
				t.Fatalf("embedded skill copy is stale for %q; run `make copy-skills`", id)
			}
			if assetDoc.Invocation != embeddedDoc.Invocation {
				t.Fatalf("invocation mismatch: asset=%q embedded=%q", assetDoc.Invocation, embeddedDoc.Invocation)
			}
		})
	}
}

func TestAllEnabledSkillAssets_StrictContractAndEmbeddedSync(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	skillsRoot := filepath.Join(repoRoot, "assets", "skills")

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read skills root: %v", err)
	}

	enabledCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		assetPath := filepath.Join(skillsRoot, id, "SKILL.md")
		if _, err := os.Stat(assetPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("stat asset skill %q: %v", id, err)
		}
		assetData, err := os.ReadFile(assetPath)
		if err != nil {
			t.Fatalf("read asset skill %q: %v", id, err)
		}

		assetDoc, err := ParseEntry(id, assetPath, assetData, Options{})
		if err != nil {
			t.Fatalf("ParseEntry permissive error for %q: %v", id, err)
		}
		if !assetDoc.Enabled {
			continue
		}
		enabledCount++

		t.Run(id, func(t *testing.T) {
			strictAssetDoc, err := ParseEntry(id, assetPath, assetData, Options{RequireContract: true})
			if err != nil {
				t.Fatalf("ParseEntry strict contract error: %v", err)
			}
			if strictAssetDoc.Manifest == nil {
				t.Fatal("expected asset manifest")
			}

			embeddedDoc, embeddedData, err := ReadEmbedded(id, Options{RequireContract: true})
			if err != nil {
				t.Fatalf("ReadEmbedded strict contract error: %v", err)
			}
			if embeddedDoc.Manifest == nil {
				t.Fatal("expected embedded manifest")
			}
			if string(assetData) != string(embeddedData) {
				t.Fatalf("embedded skill copy is stale for %q; run `make copy-skills`", id)
			}
			if strictAssetDoc.Invocation != embeddedDoc.Invocation {
				t.Fatalf("invocation mismatch: asset=%q embedded=%q", strictAssetDoc.Invocation, embeddedDoc.Invocation)
			}
		})
	}

	if enabledCount == 0 {
		t.Fatal("expected at least one enabled skill asset")
	}
}

func TestSupplementalSkills_StrictContractAndEmbeddedSync(t *testing.T) {
	supplemental := []string{
		"a11y",
		"docx",
		"xlsx",
		"pptx",
		"pdf",
		"summarize",
		"himalaya",
		"tasks",
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	for _, id := range supplemental {
		t.Run(id, func(t *testing.T) {
			assetPath := filepath.Join(repoRoot, "assets", "skills", id, "SKILL.md")
			assetData, err := os.ReadFile(assetPath)
			if err != nil {
				t.Fatalf("read asset skill: %v", err)
			}

			assetDoc, err := ParseEntry(id, assetPath, assetData, Options{RequireContract: true})
			if err != nil {
				t.Fatalf("ParseEntry asset strict contract error: %v", err)
			}
			if assetDoc.Manifest == nil {
				t.Fatal("expected asset manifest")
			}

			embeddedDoc, embeddedData, err := ReadEmbedded(id, Options{RequireContract: true})
			if err != nil {
				t.Fatalf("ReadEmbedded strict contract error: %v", err)
			}
			if embeddedDoc.Manifest == nil {
				t.Fatal("expected embedded manifest")
			}

			if string(assetData) != string(embeddedData) {
				t.Fatalf("embedded skill copy is stale for %q; run `make copy-skills`", id)
			}
			if assetDoc.Invocation != embeddedDoc.Invocation {
				t.Fatalf("invocation mismatch: asset=%q embedded=%q", assetDoc.Invocation, embeddedDoc.Invocation)
			}
		})
	}
}

func TestReadEmbedded_DOCXMentionsNativeDocumentActions(t *testing.T) {
	doc, raw, err := ReadEmbedded("docx", Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ReadEmbedded strict contract error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected embedded manifest")
	}
	content := string(raw)
	for _, needle := range []string{"blue docx action=create", "apply_template", "validate"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("embedded docx skill missing %q: %s", needle, content)
		}
	}
	if strings.Contains(content, "blue a11y action=message") {
		t.Fatalf("embedded docx skill should not inline host a11y walkthroughs: %s", content)
	}
}

func TestReadEmbedded_XLSXMentionsMutationAndAnalysisActions(t *testing.T) {
	doc, raw, err := ReadEmbedded("xlsx", Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ReadEmbedded strict contract error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected embedded manifest")
	}
	content := string(raw)
	for _, needle := range []string{"blue xlsx action=create", "append_rows", "update_cells", "sheet_compare", "Markdown"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("embedded xlsx skill missing %q: %s", needle, content)
		}
	}
}

func TestReadEmbedded_PPTXMentionsSlideAndChartActions(t *testing.T) {
	doc, raw, err := ReadEmbedded("pptx", Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ReadEmbedded strict contract error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected embedded manifest")
	}
	content := string(raw)
	for _, needle := range []string{"blue pptx action=create", "duplicate_slide", "update_chart_data", "validate_template"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("embedded pptx skill missing %q: %s", needle, content)
		}
	}
}

func TestReadEmbedded_PDFMentionsReadFillAndReformatActions(t *testing.T) {
	doc, raw, err := ReadEmbedded("pdf", Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ReadEmbedded strict contract error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected embedded manifest")
	}
	content := string(raw)
	for _, needle := range []string{"blue pdf action=read", "action=fill", "action=reformat"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("embedded pdf skill missing %q: %s", needle, content)
		}
	}
}

func TestRemovedOfficeDocsUmbrellaSkill_IsNotBundled(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	assetPath := filepath.Join(repoRoot, "assets", "skills", "office_docs", "SKILL.md")
	if _, err := os.Stat(assetPath); !os.IsNotExist(err) {
		t.Fatalf("expected office_docs umbrella asset to stay absent, err=%v", err)
	}
	if _, _, err := ReadEmbedded("office_docs", Options{}); err == nil {
		t.Fatal("expected office_docs umbrella embedded skill to stay absent")
	}
}

func TestReadEmbedded_A11yMentionsScenarioActions(t *testing.T) {
	doc, raw, err := ReadEmbedded("a11y", Options{RequireContract: true})
	if err != nil {
		t.Fatalf("ReadEmbedded strict contract error: %v", err)
	}
	if doc.Manifest == nil {
		t.Fatal("expected embedded manifest")
	}
	content := string(raw)
	for _, needle := range []string{"message", "type", "select", "click", "toggle"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("embedded a11y skill missing %q: %s", needle, content)
		}
	}
}

func TestRemovedPlaceholderSkills_AreNotBundled(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	for _, id := range []string{"timer", "datetime", "unit_converter", "search"} {
		t.Run(id, func(t *testing.T) {
			assetPath := filepath.Join(repoRoot, "assets", "skills", id, "SKILL.md")
			if _, err := os.Stat(assetPath); !os.IsNotExist(err) {
				t.Fatalf("expected removed placeholder asset %q to stay absent, err=%v", id, err)
			}
			if _, _, err := ReadEmbedded(id, Options{}); err == nil {
				t.Fatalf("expected removed placeholder embedded skill %q to stay absent", id)
			}
		})
	}
}

func TestCandidateIDs_NormalizesAndFallsBackToBaseSkill(t *testing.T) {
	got := CandidateIDs("Reminder-Add.run")
	want := []string{"Reminder-Add.run", "reminder_add.run", "Reminder-Add", "reminder_add"}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestCandidateIDs_IncludesCanonicalWebQueryForLegacyWebSearch(t *testing.T) {
	got := CandidateIDs("web_search")
	wantContains := []string{"web_search", "web_query"}
	for _, want := range wantContains {
		found := false
		for _, candidate := range got {
			if candidate == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("CandidateIDs(web_search)=%v, want to contain %q", got, want)
		}
	}
}

func TestCandidateIDs_ConfigNoLongerExpandsLegacyMgmtAlias(t *testing.T) {
	got := CandidateIDs("config")
	for _, candidate := range got {
		if candidate == "mgmt" {
			t.Fatalf("CandidateIDs(config) should not include legacy mgmt alias: %v", got)
		}
	}
}

func TestFindByCandidates_LegacyWebSearchFallsBackToEmbeddedWebQuery(t *testing.T) {
	resolved, ok := FindByCandidates(CandidateIDs("web_search"), ResolveRoots(""), Options{RequireContract: true})
	if !ok {
		t.Fatal("expected embedded web_query skill to resolve for legacy web_search lookup")
	}
	if resolved.Document.ID != "web_query" {
		t.Fatalf("resolved.Document.ID=%q, want web_query", resolved.Document.ID)
	}
	if !resolved.Embedded {
		t.Fatal("expected embedded builtin resolution")
	}
	if !strings.Contains(resolved.Source, "embedded:skills/web_query/SKILL.md") {
		t.Fatalf("resolved.Source=%q, want embedded web_query path", resolved.Source)
	}
}

func TestFindByCandidates_WorkspaceOverridesHomeAndBuiltin(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workspaceSkillDir := filepath.Join(workspaceDir, ".agents", "skills", "browser")
	if err := os.MkdirAll(workspaceSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceSkillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from workspace path
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write workspace skill: %v", err)
	}

	homeSkillDir := filepath.Join(homeDir, ".claude", "skills", "browser")
	if err := os.MkdirAll(homeSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir home skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from home path
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write home skill: %v", err)
	}

	resolved, ok := FindByCandidates(CandidateIDs("browser"), ResolveRoots(workspaceDir), Options{})
	if !ok {
		t.Fatal("expected browser skill to resolve")
	}
	if resolved.Document.Description != "Browser from workspace path" {
		t.Fatalf("description=%q, want workspace override", resolved.Document.Description)
	}
	if resolved.Source != filepath.Join(workspaceSkillDir, "SKILL.md") {
		t.Fatalf("source=%q, want workspace source", resolved.Source)
	}
	if resolved.Embedded {
		t.Fatal("expected workspace skill, got embedded")
	}
}

func TestResolvePeerRootsForManagedDir_DerivesAgentsAndClaudePeers(t *testing.T) {
	workspaceDir := t.TempDir()
	managedClaudeDir := filepath.Join(workspaceDir, ".claude", "skills")
	got := ResolvePeerRootsForManagedDir(managedClaudeDir)
	want := []string{
		filepath.Join(workspaceDir, ".agents", "skills"),
		filepath.Join(workspaceDir, ".claude", "skills"),
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}

	rawDir := filepath.Join(workspaceDir, "custom-skills")
	got = ResolvePeerRootsForManagedDir(rawDir)
	if len(got) != 1 || got[0] != rawDir {
		t.Fatalf("expected unmanaged dir to remain unchanged, got %v", got)
	}
}

func TestFindByCandidates_FallsBackToEmbeddedBuiltin(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	resolved, ok := FindByCandidates(CandidateIDs("ask"), ResolveRoots(""), Options{RequireContract: true})
	if !ok {
		t.Fatal("expected embedded ask skill to resolve")
	}
	if resolved.Document.ID != "ask" {
		t.Fatalf("resolved.Document.ID=%q, want ask", resolved.Document.ID)
	}
	if !resolved.Embedded {
		t.Fatal("expected embedded fallback")
	}
}

func TestFindByCandidates_ScanMatchesManifestNameAlias(t *testing.T) {
	workspaceDir := t.TempDir()
	aliasedDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(aliasedDir, 0o755); err != nil {
		t.Fatalf("mkdir aliased skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aliasedDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from aliased directory
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write aliased skill: %v", err)
	}

	resolved, ok := FindByCandidates(CandidateIDs("browser"), ResolveRoots(workspaceDir), Options{})
	if !ok {
		t.Fatal("expected aliased browser skill to resolve")
	}
	if resolved.Document.Description != "Browser from aliased directory" {
		t.Fatalf("description=%q, want aliased directory skill", resolved.Document.Description)
	}
	if resolved.Source != filepath.Join(aliasedDir, "SKILL.md") {
		t.Fatalf("source=%q, want aliased source", resolved.Source)
	}
}

func TestFindAnyByCandidates_IncludesDisabledWorkspaceSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo
description: Disabled demo skill
enabled: false
---
# Demo
`), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	if _, ok := FindByCandidates(CandidateIDs("demo"), ResolveRoots(workspaceDir), Options{}); ok {
		t.Fatal("expected FindByCandidates to skip disabled skill")
	}

	resolved, ok := FindAnyByCandidates(CandidateIDs("demo"), ResolveRoots(workspaceDir), Options{})
	if !ok {
		t.Fatal("expected FindAnyByCandidates to resolve disabled skill")
	}
	if resolved.Document.ID != "demo" {
		t.Fatalf("resolved.Document.ID=%q, want demo", resolved.Document.ID)
	}
	if resolved.Document.Enabled {
		t.Fatal("expected disabled document")
	}
}

func TestValidateCanonicalConflicts_DetectsAmbiguousWorkspaceDuplicates(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	agentsPath := writeCanonicalConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "team-browser", "browser", "Browser from workspace agents")
	duplicatePath := writeCanonicalConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "browser", "browser", "Browser duplicate from workspace agents")

	err := ValidateCanonicalConflicts(workspaceDir, Options{RequireContract: true}, true)
	if err == nil {
		t.Fatal("expected canonical conflict error")
	}
	if !strings.Contains(err.Error(), `workspace/.agents canonical skill "browser"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), agentsPath) || !strings.Contains(err.Error(), duplicatePath) {
		t.Fatalf("expected both conflict sources in error, got %v", err)
	}
}

func TestValidateCanonicalConflicts_AllowsWorkspaceOverrideHomeAndBuiltin(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeCanonicalConflictSkill(t, filepath.Join(workspaceDir, ".claude", "skills"), "browser", "browser", "Browser from workspace")
	writeCanonicalConflictSkill(t, filepath.Join(homeDir, ".claude", "skills"), "browser", "browser", "Browser from home")

	if err := ValidateCanonicalConflicts(workspaceDir, Options{RequireContract: true}, true); err != nil {
		t.Fatalf("expected no conflict across precedence boundaries, got %v", err)
	}
}

func TestFindAnyByCandidatesStrict_ReportsCandidateScopedCanonicalConflict(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	agentsPath := writeCanonicalConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "team-browser", "browser", "Browser from workspace agents")
	duplicatePath := writeCanonicalConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "browser", "browser", "Browser duplicate from workspace agents")
	writeCanonicalConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "reminder", "reminder", "Reminder from workspace")

	_, ok, err := FindAnyByCandidatesStrict(CandidateIDs("browser"), ResolveRoots(workspaceDir), workspaceDir, Options{RequireContract: true})
	if err == nil || ok {
		t.Fatalf("expected strict browser lookup conflict, ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), `workspace/.agents canonical skill "browser"`) {
		t.Fatalf("unexpected browser conflict error: %v", err)
	}
	if !strings.Contains(err.Error(), agentsPath) || !strings.Contains(err.Error(), duplicatePath) {
		t.Fatalf("expected both browser conflict sources in error, got %v", err)
	}

	resolved, ok, err := FindAnyByCandidatesStrict(CandidateIDs("reminder"), ResolveRoots(workspaceDir), workspaceDir, Options{RequireContract: true})
	if err != nil {
		t.Fatalf("expected unrelated reminder lookup to succeed, got %v", err)
	}
	if !ok {
		t.Fatal("expected reminder lookup to resolve")
	}
	if resolved.Document.ID != "reminder" {
		t.Fatalf("resolved.Document.ID=%q, want reminder", resolved.Document.ID)
	}
}

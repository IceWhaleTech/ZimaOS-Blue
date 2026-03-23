package skillmarket

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newBatchTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	store, err := NewStore(db)
	if err != nil {
		db.Close()
		t.Fatalf("NewStore() error = %v", err)
	}
	return store, db
}

func batchTestRecord(id, sourceID, version, description string) *SkillUpsertRecord {
	return &SkillUpsertRecord{
		Doc: &SkillDocument{
			ID:            id,
			Slug:          id,
			Name:          id,
			Description:   description,
			LatestVersion: version,
			Installable:   true,
			InstallType:   InstallTypeRawSkill,
			ArtifactKind:  ArtifactKindOpenSource,
			Published:     true,
			SourceID:      sourceID,
			SourceName:    sourceID,
			SourceGroup:   sourceID,
			SourceType:    "fixture",
			DownloadURL:   "https://example.com/" + id + "/SKILL.md",
			SkillPath:     "SKILL.md",
			SkillContent:  "# " + id,
		},
		Version: &SkillVersion{
			SkillID:    id,
			Version:    version,
			SourceURL:  "https://example.com/" + id + "/SKILL.md",
			Checksum:   version + "-" + id,
			SkillPath:  "SKILL.md",
			RawSkillMD: "# " + id,
		},
		Report: &SecurityReport{
			Score:               92,
			RiskLevel:           RiskLow,
			SecurityBadge:       BadgeGreen,
			VulnerabilityStatus: VulnerabilityStatusNotApplicable,
			InstallSurface: InstallSurface{
				InstallType:  InstallTypeRawSkill,
				ArtifactKind: ArtifactKindOpenSource,
				Installable:  true,
			},
			ScannerVersion: ScannerVersion,
			LLMStatus:      "skipped",
		},
	}
}

func TestStoreUpsertSkillBatchInsertsSkillsVersionsAndReports(t *testing.T) {
	store, db := newBatchTestStore(t)
	defer db.Close()

	ctx := context.Background()
	if err := store.UpsertSource(ctx, Source{ID: "fixture", Type: "fixture", BaseURL: "https://example.com", Enabled: true, Priority: 10}); err != nil {
		t.Fatalf("UpsertSource() error = %v", err)
	}

	result, err := store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{
		batchTestRecord("alpha-skill", "fixture", "1.0.0", "alpha"),
		batchTestRecord("beta-skill", "fixture", "1.1.0", "beta"),
	})
	if err != nil {
		t.Fatalf("UpsertSkillBatch() error = %v", err)
	}
	if result.Inserted != 2 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected batch result: %+v", result)
	}

	detail, err := store.GetSkill(ctx, "alpha-skill")
	if err != nil {
		t.Fatalf("GetSkill(alpha-skill) error = %v", err)
	}
	if detail == nil || detail.Version == nil || detail.Security == nil {
		t.Fatalf("expected alpha-skill detail/version/security, got %+v", detail)
	}

	var versionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skill_versions`).Scan(&versionCount); err != nil {
		t.Fatalf("count skill_versions: %v", err)
	}
	if versionCount != 2 {
		t.Fatalf("skill_versions count = %d, want 2", versionCount)
	}

	var reportCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skill_security_reports`).Scan(&reportCount); err != nil {
		t.Fatalf("count skill_security_reports: %v", err)
	}
	if reportCount != 2 {
		t.Fatalf("skill_security_reports count = %d, want 2", reportCount)
	}
}

func TestStoreUpsertSkillBatchMixedInsertUpdateAndSkip(t *testing.T) {
	store, db := newBatchTestStore(t)
	defer db.Close()

	ctx := context.Background()
	for _, source := range []Source{
		{ID: "tencent-skillhub", Type: "lightmake_api", BaseURL: "https://lightmake.site", Enabled: true, Priority: 5},
		{ID: "clawhub", Type: "clawhub", BaseURL: "https://www.clawhub.ai", Enabled: true, Priority: 10},
	} {
		if err := store.UpsertSource(ctx, source); err != nil {
			t.Fatalf("UpsertSource(%s) error = %v", source.ID, err)
		}
	}

	if err := store.UpsertSkill(ctx, batchTestRecord("update-me", "tencent-skillhub", "1.0.0", "before update").Doc, batchTestRecord("update-me", "tencent-skillhub", "1.0.0", "before update").Version, batchTestRecord("update-me", "tencent-skillhub", "1.0.0", "before update").Report); err != nil {
		t.Fatalf("seed update-me error = %v", err)
	}
	if err := store.UpsertSkill(ctx, batchTestRecord("skip-me", "tencent-skillhub", "1.0.0", "keep me").Doc, batchTestRecord("skip-me", "tencent-skillhub", "1.0.0", "keep me").Version, batchTestRecord("skip-me", "tencent-skillhub", "1.0.0", "keep me").Report); err != nil {
		t.Fatalf("seed skip-me error = %v", err)
	}

	updateRecord := batchTestRecord("update-me", "tencent-skillhub", "2.0.0", "after update")
	insertRecord := batchTestRecord("new-skill", "tencent-skillhub", "1.0.0", "brand new")
	skipRecord := batchTestRecord("skip-me", "clawhub", "2.0.0", "should not win")

	result, err := store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{updateRecord, insertRecord, skipRecord})
	if err != nil {
		t.Fatalf("UpsertSkillBatch() error = %v", err)
	}
	if result.Inserted != 1 || result.Updated != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected batch result: %+v", result)
	}

	updatedDetail, err := store.GetSkill(ctx, "update-me")
	if err != nil {
		t.Fatalf("GetSkill(update-me) error = %v", err)
	}
	if updatedDetail.Skill.Description != "after update" {
		t.Fatalf("update-me description = %q, want after update", updatedDetail.Skill.Description)
	}
	if updatedDetail.Skill.LatestVersion != "2.0.0" {
		t.Fatalf("update-me latest_version = %q, want 2.0.0", updatedDetail.Skill.LatestVersion)
	}

	skippedDetail, err := store.GetSkill(ctx, "skip-me")
	if err != nil {
		t.Fatalf("GetSkill(skip-me) error = %v", err)
	}
	if skippedDetail.Skill.Description != "keep me" {
		t.Fatalf("skip-me description = %q, want keep me", skippedDetail.Skill.Description)
	}
	if skippedDetail.Skill.SourceID != "tencent-skillhub" {
		t.Fatalf("skip-me source id = %q, want tencent-skillhub", skippedDetail.Skill.SourceID)
	}
}

func TestStoreUpsertSkillBatchHandlesHundredRecords(t *testing.T) {
	store, db := newBatchTestStore(t)
	defer db.Close()

	ctx := context.Background()
	if err := store.UpsertSource(ctx, Source{ID: "fixture", Type: "fixture", BaseURL: "https://example.com", Enabled: true, Priority: 10}); err != nil {
		t.Fatalf("UpsertSource() error = %v", err)
	}

	records := make([]*SkillUpsertRecord, 0, 100)
	for i := 0; i < 100; i++ {
		records = append(records, batchTestRecord(
			fmt.Sprintf("skill-%03d", i),
			"fixture",
			"1.0.0",
			fmt.Sprintf("skill %d", i),
		))
	}

	result, err := store.UpsertSkillBatch(ctx, records)
	if err != nil {
		t.Fatalf("UpsertSkillBatch() error = %v", err)
	}
	if result.Inserted != 100 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected batch result: %+v", result)
	}

	var skillCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skills`).Scan(&skillCount); err != nil {
		t.Fatalf("count skills: %v", err)
	}
	if skillCount != 100 {
		t.Fatalf("skills count = %d, want 100", skillCount)
	}
}

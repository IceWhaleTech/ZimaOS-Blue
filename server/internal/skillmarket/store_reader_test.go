package skillmarket

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreUsesReaderDBForSourceAndInstallReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skillmarket-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	t.Cleanup(func() {
		if writeDB != nil {
			_ = writeDB.Close()
		}
	})

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected dedicated reader db")
	}

	ctx := context.Background()
	source := Source{
		ID:          "fixture-source",
		Type:        "fixture",
		BaseURL:     "https://example.com",
		DisplayName: "Fixture Source",
		SourceGroup: "fixture",
		Enabled:     true,
		Priority:    5,
		Headers: map[string]string{
			"Authorization": "Bearer fixture",
		},
	}
	if err := store.UpsertSource(ctx, source); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	record := batchTestRecord("git-expert", source.ID, "1.2.3", "Expert git workflows")
	record.Doc.Category = "development"
	record.Doc.Tags = []string{"git"}
	record.Doc.LatestVersion = "1.3.0"
	record.Doc.SourceName = source.DisplayName
	record.Doc.SourceGroup = source.SourceGroup
	record.Doc.SourceType = source.Type
	if err := store.UpsertSkill(ctx, record.Doc, record.Version, record.Report); err != nil {
		t.Fatalf("UpsertSkill: %v", err)
	}

	if err := store.SetInstalledSkill(ctx, InstalledSkill{
		SkillID:           record.Doc.ID,
		InstalledVersion:  record.Version.Version,
		Checksum:          "checksum-123",
		SourceURL:         record.Version.SourceURL,
		Enabled:           true,
		AutoUpdate:        true,
		LastSecurityScore: 92,
	}); err != nil {
		t.Fatalf("SetInstalledSkill: %v", err)
	}

	if err := store.RecordUpdateCheck(ctx, AvailableUpdate{
		SkillID:        record.Doc.ID,
		LatestVersion:  "1.3.0",
		LatestChecksum: "checksum-130",
		Action:         "upgrade",
	}); err != nil {
		t.Fatalf("RecordUpdateCheck: %v", err)
	}

	if err := store.ReplaceCurations(ctx, []SkillCuration{{
		SkillID:      record.Doc.ID,
		FeaturedRank: 1,
		BoostWeight:  1.5,
		Label:        "Editor's pick",
		Reason:       "reader fixture",
	}}); err != nil {
		t.Fatalf("ReplaceCurations: %v", err)
	}

	if err := store.UpsertCurationSyncState(ctx, CurationSyncState{
		ID:        "default",
		SourceURL: "https://example.com/curations.yaml",
		Checksum:  "curation-sha",
	}); err != nil {
		t.Fatalf("UpsertCurationSyncState: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}
	writeDB = nil

	sources, err := store.ListSources(ctx)
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != source.ID {
		t.Fatalf("unexpected sources via reader: %+v", sources)
	}

	trending, err := store.ListTrending(ctx, "development", 10)
	if err != nil {
		t.Fatalf("ListTrending: %v", err)
	}
	if len(trending) != 1 || trending[0].ID != record.Doc.ID {
		t.Fatalf("unexpected trending via reader: %+v", trending)
	}

	featured, err := store.ListFeatured(ctx, "development", source.ID, 10)
	if err != nil {
		t.Fatalf("ListFeatured: %v", err)
	}
	if len(featured) != 1 || featured[0].ID != record.Doc.ID || featured[0].CuratedRank != 1 {
		t.Fatalf("unexpected featured via reader: %+v", featured)
	}

	filtered, err := store.ListFilteredSkills(ctx, SearchQuery{
		Category: "development",
		Sort:     "trending",
	})
	if err != nil {
		t.Fatalf("ListFilteredSkills: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != record.Doc.ID {
		t.Fatalf("unexpected filtered skills via reader: %+v", filtered)
	}

	detail, err := store.GetSkill(ctx, record.Doc.ID)
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if detail == nil || detail.Version == nil || detail.Security == nil {
		t.Fatalf("unexpected skill detail via reader: %+v", detail)
	}
	if detail.Skill.ID != record.Doc.ID || detail.Version.Version != "1.2.3" || detail.Security.Score != 92 {
		t.Fatalf("unexpected populated detail via reader: %+v", detail)
	}

	latest, err := store.GetLatestVersion(ctx, record.Doc.ID)
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if latest.Version != "1.2.3" {
		t.Fatalf("latest version via reader = %+v, want 1.2.3", latest)
	}

	explicitVersion, err := store.GetSkillVersion(ctx, record.Doc.ID, record.Version.Version)
	if err != nil {
		t.Fatalf("GetSkillVersion: %v", err)
	}
	if explicitVersion.ID == "" || explicitVersion.Version != record.Version.Version {
		t.Fatalf("unexpected explicit version via reader: %+v", explicitVersion)
	}

	report, err := store.GetSecurityReport(ctx, record.Doc.ID, record.Version.Version)
	if err != nil {
		t.Fatalf("GetSecurityReport: %v", err)
	}
	if report.SkillID != record.Doc.ID || report.Version != record.Version.Version || report.Score != 92 {
		t.Fatalf("unexpected security report via reader: %+v", report)
	}

	installed, err := store.GetInstalledSkill(ctx, record.Doc.ID)
	if err != nil {
		t.Fatalf("GetInstalledSkill: %v", err)
	}
	if installed.SkillID != record.Doc.ID || !installed.Enabled || !installed.AutoUpdate {
		t.Fatalf("unexpected installed skill via reader: %+v", installed)
	}

	installedList, err := store.ListInstalledSkills(ctx)
	if err != nil {
		t.Fatalf("ListInstalledSkills: %v", err)
	}
	if len(installedList) != 1 {
		t.Fatalf("installed list len = %d, want 1", len(installedList))
	}
	if installedList[0].Name != record.Doc.Name || !installedList[0].UpdateAvailable {
		t.Fatalf("unexpected installed list via reader: %+v", installedList)
	}

	updates, err := store.ListUpdates(ctx)
	if err != nil {
		t.Fatalf("ListUpdates: %v", err)
	}
	if len(updates) != 1 || updates[0].LatestVersion != "1.3.0" {
		t.Fatalf("unexpected updates via reader: %+v", updates)
	}

	filters, err := store.GetFilters(ctx)
	if err != nil {
		t.Fatalf("GetFilters: %v", err)
	}
	if len(filters.Categories) != 1 || filters.Categories[0].Value != "development" {
		t.Fatalf("unexpected filters via reader: %+v", filters.Categories)
	}

	state, err := store.GetCurationSyncState(ctx, "default")
	if err != nil {
		t.Fatalf("GetCurationSyncState: %v", err)
	}
	if state.SourceURL != "https://example.com/curations.yaml" || state.Checksum != "curation-sha" {
		t.Fatalf("unexpected curation sync state via reader: %+v", state)
	}

	store.ftsEnabled = false

	browse, err := store.Search(ctx, SearchQuery{
		Category: "development",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search(browse): %v", err)
	}
	if browse.Total != 1 || len(browse.Skills) != 1 || browse.Skills[0].Skill.ID != record.Doc.ID {
		t.Fatalf("unexpected browse search via reader: %+v", browse)
	}

	search, err := store.Search(ctx, SearchQuery{
		Query:    "expert",
		Category: "development",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search(fallback): %v", err)
	}
	if search.Total != 1 || len(search.Skills) != 1 || search.Skills[0].Skill.ID != record.Doc.ID || search.Skills[0].MatchSource != "rank" {
		t.Fatalf("unexpected fallback search via reader: %+v", search)
	}

	candidates, err := store.SearchKeywordCandidates(ctx, SearchQuery{
		Query:    "expert",
		Category: "development",
	})
	if err != nil {
		t.Fatalf("SearchKeywordCandidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Skill.ID != record.Doc.ID || candidates[0].MatchSource != "rank" {
		t.Fatalf("unexpected keyword candidates via reader: %+v", candidates)
	}
}

func TestStoreListUpdatesFiltersUnchangedVersionsViaReader(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skillmarket-updates-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	t.Cleanup(func() {
		if writeDB != nil {
			_ = writeDB.Close()
		}
	})

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}

	ctx := context.Background()
	if err := store.SetInstalledSkill(ctx, InstalledSkill{
		SkillID:          "needs-update",
		InstalledVersion: "1.0.0",
		Checksum:         "checksum-100",
		Enabled:          true,
	}); err != nil {
		t.Fatalf("SetInstalledSkill(needs-update): %v", err)
	}
	if err := store.RecordUpdateCheck(ctx, AvailableUpdate{
		SkillID:        "needs-update",
		LatestVersion:  "1.1.0",
		LatestChecksum: "checksum-110",
		Action:         "upgrade",
	}); err != nil {
		t.Fatalf("RecordUpdateCheck(needs-update): %v", err)
	}

	if err := store.SetInstalledSkill(ctx, InstalledSkill{
		SkillID:          "already-current",
		InstalledVersion: "2.0.0",
		Checksum:         "checksum-200",
		Enabled:          true,
	}); err != nil {
		t.Fatalf("SetInstalledSkill(already-current): %v", err)
	}
	if err := store.RecordUpdateCheck(ctx, AvailableUpdate{
		SkillID:        "already-current",
		LatestVersion:  "2.0.0",
		LatestChecksum: "checksum-200",
		Action:         "noop",
	}); err != nil {
		t.Fatalf("RecordUpdateCheck(already-current): %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}
	writeDB = nil

	updates, err := store.ListUpdates(ctx)
	if err != nil {
		t.Fatalf("ListUpdates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("updates len = %d, want 1", len(updates))
	}
	if updates[0].SkillID != "needs-update" || updates[0].LatestVersion != "1.1.0" || updates[0].Action != "upgrade" {
		t.Fatalf("unexpected filtered updates via reader: %+v", updates)
	}
}

func TestStoreFTSSearchUsesReaderDBAndHonorsPublishedFilters(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "skillmarket-fts-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	t.Cleanup(func() {
		if writeDB != nil {
			_ = writeDB.Close()
		}
	})

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}
	if !store.ftsEnabled {
		t.Skip("fts is not available in this sqlite build")
	}

	ctx := context.Background()
	if err := store.UpsertSource(ctx, Source{
		ID:          "fixture",
		Type:        "fixture",
		BaseURL:     "https://example.com",
		DisplayName: "Fixture Source",
		SourceGroup: "fixture",
		Enabled:     true,
		Priority:    1,
	}); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	visible := batchTestRecord("visible-skill", "fixture", "1.0.0", "Visible workflow expert")
	visible.Doc.Name = "Visible Workflow"
	visible.Doc.Category = "development"
	visible.Doc.Tags = []string{"workflow", "visible"}
	visible.Doc.SkillContent = "# Visible Workflow\nworkflow workflow helper"
	if err := store.UpsertSkill(ctx, visible.Doc, visible.Version, visible.Report); err != nil {
		t.Fatalf("UpsertSkill(visible): %v", err)
	}

	hidden := batchTestRecord("hidden-skill", "fixture", "1.0.0", "Hidden workflow expert")
	hidden.Doc.Name = "Hidden Workflow"
	hidden.Doc.Category = "development"
	hidden.Doc.Tags = []string{"workflow", "hidden"}
	hidden.Doc.SkillContent = "# Hidden Workflow\nworkflow workflow workflow hidden"
	if err := store.UpsertSkill(ctx, hidden.Doc, hidden.Version, hidden.Report); err != nil {
		t.Fatalf("UpsertSkill(hidden): %v", err)
	}

	if _, err := writeDB.ExecContext(ctx, `UPDATE skills SET published = 0 WHERE id = ?`, hidden.Doc.ID); err != nil {
		t.Fatalf("hide skill: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write db: %v", err)
	}
	writeDB = nil

	result, err := store.Search(ctx, SearchQuery{
		Query:    "workflow",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search(fts): %v", err)
	}
	if result.Total != 1 || len(result.Skills) != 1 {
		t.Fatalf("unexpected fts search result set: %+v", result)
	}
	if result.Skills[0].Skill.ID != visible.Doc.ID || result.Skills[0].MatchSource != "fts5" {
		t.Fatalf("unexpected visible search hit: %+v", result.Skills[0])
	}

	candidates, err := store.SearchKeywordCandidates(ctx, SearchQuery{Query: "workflow"})
	if err != nil {
		t.Fatalf("SearchKeywordCandidates(fts): %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("keyword candidates len = %d, want 1", len(candidates))
	}
	if candidates[0].Skill.ID != visible.Doc.ID || candidates[0].MatchSource != "fts5" {
		t.Fatalf("unexpected keyword candidate: %+v", candidates[0])
	}
}

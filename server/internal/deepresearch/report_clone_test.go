package deepresearch

import "testing"

func TestCloneReport_PreservesRecentRetrievalFields(t *testing.T) {
	original := &Report{
		Answer:           "Recent discussion is mostly positive.",
		RetrievalProfile: "recent_multi_site_v1",
		LookbackDays:     30,
		BrowserAssisted:  true,
		ItemsBySource: map[string]interface{}{
			"reddit": []interface{}{
				map[string]interface{}{"title": "Thread", "url": "https://reddit.com/r/test"},
			},
		},
		ErrorsBySource: map[string]string{
			"github": "rate limited",
		},
		Clusters: []map[string]interface{}{
			{"label": "performance", "item_count": 2},
		},
	}

	cloned := cloneReport(original)
	if cloned == nil {
		t.Fatal("expected cloned report")
	}
	if cloned.RetrievalProfile != "recent_multi_site_v1" {
		t.Fatalf("retrieval_profile = %q, want recent_multi_site_v1", cloned.RetrievalProfile)
	}
	if cloned.LookbackDays != 30 {
		t.Fatalf("lookback_days = %d, want 30", cloned.LookbackDays)
	}
	if !cloned.BrowserAssisted {
		t.Fatal("expected browser_assisted to stay true")
	}
	if got := cloned.ErrorsBySource["github"]; got != "rate limited" {
		t.Fatalf("errors_by_source.github = %q, want rate limited", got)
	}
	if len(cloned.ItemsBySource) != 1 || len(cloned.Clusters) != 1 {
		t.Fatalf("unexpected cloned structured fields: %#v %#v", cloned.ItemsBySource, cloned.Clusters)
	}

	cloned.ErrorsBySource["github"] = "changed"
	cloned.ItemsBySource["reddit"] = []interface{}{}
	cloned.Clusters[0]["label"] = "changed"
	if original.ErrorsBySource["github"] != "rate limited" {
		t.Fatalf("original errors_by_source mutated: %#v", original.ErrorsBySource)
	}
	if redditItems, _ := original.ItemsBySource["reddit"].([]interface{}); len(redditItems) != 1 {
		t.Fatalf("original items_by_source mutated: %#v", original.ItemsBySource)
	}
	if original.Clusters[0]["label"] != "performance" {
		t.Fatalf("original clusters mutated: %#v", original.Clusters)
	}
}

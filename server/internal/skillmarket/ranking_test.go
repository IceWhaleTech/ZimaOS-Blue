package skillmarket

import (
	"testing"
	"time"
)

func TestComputeTrendingScoreRewardsFreshness(t *testing.T) {
	now := time.Now()
	fresh := computeTrendingScore(100, 500, now.Add(-24*time.Hour), 90)
	stale := computeTrendingScore(100, 500, now.Add(-180*24*time.Hour), 90)
	if fresh <= stale {
		t.Fatalf("fresh score = %f, stale score = %f, want fresh > stale", fresh, stale)
	}
}

func TestComputePopularityScoreRewardsDownloads(t *testing.T) {
	low := computePopularityScore(10, 100)
	high := computePopularityScore(10, 10000)
	if high <= low {
		t.Fatalf("high popularity = %f, low popularity = %f, want high > low", high, low)
	}
}

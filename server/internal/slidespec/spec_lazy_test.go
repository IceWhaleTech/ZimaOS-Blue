package slidespec

import (
	"sync"
	"testing"
)

func TestSlidespecRegexes_InitializeOnDemand(t *testing.T) {
	originalBulletPrefixRE := bulletPrefixRE
	originalBulletPrefixREOnce := bulletPrefixREOnce
	originalMetricValueTokenRE := metricValueTokenRE
	originalMetricValueTokenREOnce := metricValueTokenREOnce

	bulletPrefixRE = nil
	bulletPrefixREOnce = sync.Once{}
	metricValueTokenRE = nil
	metricValueTokenREOnce = sync.Once{}
	t.Cleanup(func() {
		bulletPrefixRE = originalBulletPrefixRE
		bulletPrefixREOnce = originalBulletPrefixREOnce
		metricValueTokenRE = originalMetricValueTokenRE
		metricValueTokenREOnce = originalMetricValueTokenREOnce
	})

	if bulletPrefixRE != nil || metricValueTokenRE != nil {
		t.Fatal("expected slidespec regexes to start nil")
	}

	if got := cleanBulletLine("1. Growth 18"); got != "Growth 18" {
		t.Fatalf("cleanBulletLine() = %q, want %q", got, "Growth 18")
	}
	if bulletPrefixRE == nil {
		t.Fatal("expected bullet prefix regex to initialize on first cleanBulletLine call")
	}
	if metricValueTokenRE != nil {
		t.Fatal("expected metric regex to remain nil until metric detection runs")
	}

	if !looksLikeMetricBullet("Growth 18") {
		t.Fatal("expected looksLikeMetricBullet to recognize metric bullet")
	}
	if metricValueTokenRE == nil {
		t.Fatal("expected metric regex to initialize on first looksLikeMetricBullet call")
	}
}

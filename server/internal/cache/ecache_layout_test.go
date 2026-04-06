package cache

import "testing"

func TestComputeGenericCacheLayout(t *testing.T) {
	tests := []struct {
		name            string
		maxSize         int
		wantBucketCount uint16
		wantBucketSize  uint16
		wantSecondLevel uint16
	}{
		{
			name:            "tiny cache uses single bucket",
			maxSize:         10,
			wantBucketCount: 1,
			wantBucketSize:  10,
			wantSecondLevel: 2,
		},
		{
			name:            "small cache stays single bucket",
			maxSize:         50,
			wantBucketCount: 1,
			wantBucketSize:  50,
			wantSecondLevel: 12,
		},
		{
			name:            "medium cache scales bucket count",
			maxSize:         200,
			wantBucketCount: 4,
			wantBucketSize:  50,
			wantSecondLevel: 12,
		},
		{
			name:            "default sized cache keeps wider sharding",
			maxSize:         1000,
			wantBucketCount: 16,
			wantBucketSize:  63,
			wantSecondLevel: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketCount, bucketSize, secondLevelSize := computeGenericCacheLayout(tt.maxSize)
			if bucketCount != tt.wantBucketCount {
				t.Fatalf("bucketCount = %d, want %d", bucketCount, tt.wantBucketCount)
			}
			if bucketSize != tt.wantBucketSize {
				t.Fatalf("bucketSize = %d, want %d", bucketSize, tt.wantBucketSize)
			}
			if secondLevelSize != tt.wantSecondLevel {
				t.Fatalf("secondLevelSize = %d, want %d", secondLevelSize, tt.wantSecondLevel)
			}
		})
	}
}

func TestNewGenericCacheWithStats_SmallCacheStillWorks(t *testing.T) {
	cache := NewGenericCacheWithStats(Config{
		MaxSize:    10,
		DefaultTTL: 0,
	}, "test_small_cache")

	cache.Put("a", "value")
	got, ok := cache.Get("a")
	if !ok {
		t.Fatal("expected cached value")
	}
	if got != "value" {
		t.Fatalf("cache value = %v, want %v", got, "value")
	}
}

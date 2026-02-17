package timeutil

import (
	"testing"
	"time"
)

func TestNow(t *testing.T) {
	cached := Now()
	real := time.Now().Unix()

	diff := real - cached
	if diff < -1 || diff > 1 {
		t.Errorf("Now() = %d, want within 1 second of %d, diff = %d", cached, real, diff)
	}
}

func TestNowNano(t *testing.T) {
	cached := NowNano()
	real := time.Now().UnixNano()

	diff := real - cached
	maxDiff := int64(200 * time.Millisecond)
	if diff < -maxDiff || diff > maxDiff {
		t.Errorf("NowNano() diff = %d ns, want within %d ns", diff, maxDiff)
	}
}

func TestNowMilli(t *testing.T) {
	cached := NowMilli()
	real := time.Now().UnixMilli()

	diff := real - cached
	if diff < -200 || diff > 200 {
		t.Errorf("NowMilli() = %d, want within 200ms of %d, diff = %d", cached, real, diff)
	}
}

func TestNowTime(t *testing.T) {
	cached := NowTime()
	real := time.Now()

	diff := real.Sub(cached)
	if diff < -200*time.Millisecond || diff > 200*time.Millisecond {
		t.Errorf("NowTime() diff = %v, want within 200ms", diff)
	}
}

func TestMonotonic(t *testing.T) {
	prev := Monotonic()
	time.Sleep(150 * time.Millisecond)
	next := Monotonic()
	if next < prev {
		t.Errorf("Monotonic() went backwards: %d -> %d", prev, next)
	}
}

func TestSince(t *testing.T) {
	start := NowNano()
	time.Sleep(150 * time.Millisecond)
	elapsed := Since(start)

	if elapsed < 0 || elapsed > 500*time.Millisecond {
		t.Errorf("Since() = %v, want between 0 and 500ms", elapsed)
	}
}

func TestSinceTime(t *testing.T) {
	start := time.Now()
	time.Sleep(50 * time.Millisecond)
	elapsed := SinceTime(start)

	if elapsed < -100*time.Millisecond || elapsed > 300*time.Millisecond {
		t.Errorf("SinceTime() = %v, want between -100ms and 300ms", elapsed)
	}
}

func BenchmarkNow(b *testing.B) {
	b.Run("CachedNow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Now()
		}
	})
	b.Run("RealTimeNow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Now().Unix()
		}
	})
}

func BenchmarkNowNano(b *testing.B) {
	b.Run("CachedNowNano", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NowNano()
		}
	})
	b.Run("RealTimeNowNano", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Now().UnixNano()
		}
	})
}

func BenchmarkNowTime(b *testing.B) {
	b.Run("CachedNowTime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NowTime()
		}
	})
	b.Run("RealTimeNow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Now()
		}
	})
}

func BenchmarkMonotonic(b *testing.B) {
	b.Run("Monotonic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Monotonic()
		}
	})
}

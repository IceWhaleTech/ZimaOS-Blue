package bootstrap

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestNormalizeStartupMemoryTrimDelays(t *testing.T) {
	got := normalizeStartupMemoryTrimDelays([]time.Duration{
		20 * time.Second,
		0,
		8 * time.Second,
		20 * time.Second,
		-1 * time.Second,
		8 * time.Second,
	})
	want := []time.Duration{8 * time.Second, 20 * time.Second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeStartupMemoryTrimDelays() = %v, want %v", got, want)
	}
}

func TestRunStartupMemoryTrimLoopUsesRelativeWaits(t *testing.T) {
	var waits []time.Duration
	var trims []time.Duration

	runStartupMemoryTrimLoop(
		context.Background(),
		normalizeStartupMemoryTrimDelays([]time.Duration{
			20 * time.Second,
			8 * time.Second,
			20 * time.Second,
		}),
		func(delay time.Duration) {
			trims = append(trims, delay)
		},
		func(ctx context.Context, delay time.Duration) bool {
			waits = append(waits, delay)
			return true
		},
	)

	if want := []time.Duration{8 * time.Second, 12 * time.Second}; !reflect.DeepEqual(waits, want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
	if want := []time.Duration{8 * time.Second, 20 * time.Second}; !reflect.DeepEqual(trims, want) {
		t.Fatalf("trims = %v, want %v", trims, want)
	}
}

func TestRunStartupMemoryTrimLoopStopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var trims []time.Duration
	runStartupMemoryTrimLoop(
		ctx,
		[]time.Duration{8 * time.Second, 20 * time.Second},
		func(delay time.Duration) {
			trims = append(trims, delay)
		},
		func(ctx context.Context, delay time.Duration) bool {
			return ctx.Err() == nil
		},
	)

	if len(trims) != 0 {
		t.Fatalf("expected no trims after cancellation, got %v", trims)
	}
}

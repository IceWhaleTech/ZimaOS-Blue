package browser

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestSafeRodPageCallRecoversPanics(t *testing.T) {
	err := safeRodPageCall("wait load", func() error {
		panic("assignment to entry in nil map")
	})
	if err == nil {
		t.Fatal("expected panic to be converted into an error")
	}
	if !strings.Contains(err.Error(), "rod panic during wait load") {
		t.Fatalf("error = %q, want rod panic marker", err.Error())
	}
}

func TestSafeRodPageResultRecoversPanics(t *testing.T) {
	_, err := safeRodPageResult("extract element lookup", func() (string, error) {
		panic("assignment to entry in nil map")
	})
	if err == nil {
		t.Fatal("expected panic to be converted into an error")
	}
	if !strings.Contains(err.Error(), "rod panic during extract element lookup") {
		t.Fatalf("error = %q, want rod panic marker", err.Error())
	}
}

func TestIsConnectionClosedTreatsRodWaitPanicsAsRecoverable(t *testing.T) {
	err := safeRodPageCall("wait stable", func() error {
		panic("assignment to entry in nil map")
	})
	if !isConnectionClosed(err) {
		t.Fatalf("isConnectionClosed(%v) = false, want true", err)
	}
}

func TestWaitPageStableUsesLoadAndIdleApproximation(t *testing.T) {
	originalLoad := waitPageStableLoadFn
	originalIdle := waitPageStableIdleFn
	originalSleep := waitPageStableSleep
	defer func() {
		waitPageStableLoadFn = originalLoad
		waitPageStableIdleFn = originalIdle
		waitPageStableSleep = originalSleep
	}()

	var loadCalled, idleCalled, sleepCalled bool
	waitPageStableLoadFn = func(page *rod.Page, timeout time.Duration) error {
		loadCalled = page != nil && timeout == 800*time.Millisecond
		return nil
	}
	waitPageStableIdleFn = func(page *rod.Page, timeout time.Duration) error {
		idleCalled = page != nil && timeout == 400*time.Millisecond
		return nil
	}
	waitPageStableSleep = func(time.Duration) {
		sleepCalled = true
	}

	if err := waitPageStable(&rod.Page{}, 800*time.Millisecond); err != nil {
		t.Fatalf("waitPageStable() error = %v", err)
	}
	if !loadCalled {
		t.Fatal("expected load approximation to run")
	}
	if !idleCalled {
		t.Fatal("expected idle approximation to run")
	}
	if sleepCalled {
		t.Fatal("did not expect settle sleep when idle wait succeeds")
	}
}

func TestWaitPageStableFallsBackToSleepWhenIdleWaitFails(t *testing.T) {
	originalLoad := waitPageStableLoadFn
	originalIdle := waitPageStableIdleFn
	originalSleep := waitPageStableSleep
	defer func() {
		waitPageStableLoadFn = originalLoad
		waitPageStableIdleFn = originalIdle
		waitPageStableSleep = originalSleep
	}()

	var sleptFor time.Duration
	waitPageStableLoadFn = func(*rod.Page, time.Duration) error { return nil }
	waitPageStableIdleFn = func(*rod.Page, time.Duration) error { return errors.New("idle probe failed") }
	waitPageStableSleep = func(d time.Duration) {
		sleptFor = d
	}

	if err := waitPageStable(&rod.Page{}, 2*time.Second); err != nil {
		t.Fatalf("waitPageStable() error = %v", err)
	}
	if sleptFor != 500*time.Millisecond {
		t.Fatalf("sleptFor = %v, want 500ms capped settle delay", sleptFor)
	}
}

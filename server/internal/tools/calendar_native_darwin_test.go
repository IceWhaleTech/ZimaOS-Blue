//go:build darwin

package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func restoreDarwinCalendarAccessFuncs() func() {
	prevSupportsFullAccess := darwinCalendarSupportsFullAccessRequestFunc
	prevHasUsage := darwinCalendarHasFullAccessUsageDescriptionFunc
	prevHasLegacyUsage := darwinCalendarHasLegacyUsageDescriptionFunc
	prevStatus := darwinCalendarAuthorizationStatusFunc
	prevRequest := darwinCalendarRequestFullAccessFunc
	return func() {
		darwinCalendarSupportsFullAccessRequestFunc = prevSupportsFullAccess
		darwinCalendarHasFullAccessUsageDescriptionFunc = prevHasUsage
		darwinCalendarHasLegacyUsageDescriptionFunc = prevHasLegacyUsage
		darwinCalendarAuthorizationStatusFunc = prevStatus
		darwinCalendarRequestFullAccessFunc = prevRequest
	}
}

func TestCheckDarwinCalendarAuthorizationRequestShortCircuitsWhenUsageMissing(t *testing.T) {
	restore := restoreDarwinCalendarAccessFuncs()
	defer restore()

	darwinCalendarSupportsFullAccessRequestFunc = func() bool { return true }
	darwinCalendarHasFullAccessUsageDescriptionFunc = func() bool { return false }
	darwinCalendarHasLegacyUsageDescriptionFunc = func() bool { return false }
	darwinCalendarAuthorizationStatusFunc = func() (int, error) {
		t.Fatal("authorization status should not run when usage description is missing")
		return 0, nil
	}
	darwinCalendarRequestFullAccessFunc = func() (bool, error) {
		t.Fatal("requestFullAccess should not run when usage description is missing")
		return false, nil
	}

	err := checkDarwinCalendarAuthorizationRequest()
	if err == nil {
		t.Fatal("expected error when calendar usage description is missing")
	}
	if !strings.Contains(err.Error(), "NSCalendarsFullAccessUsageDescription") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckDarwinCalendarAuthorizationRequestRequiresLegacyUsageDescriptionWhenNeeded(t *testing.T) {
	restore := restoreDarwinCalendarAccessFuncs()
	defer restore()

	darwinCalendarSupportsFullAccessRequestFunc = func() bool { return false }
	darwinCalendarHasFullAccessUsageDescriptionFunc = func() bool { return false }
	darwinCalendarHasLegacyUsageDescriptionFunc = func() bool { return false }
	darwinCalendarAuthorizationStatusFunc = func() (int, error) {
		t.Fatal("authorization status should not run when legacy usage description is missing")
		return 0, nil
	}
	darwinCalendarRequestFullAccessFunc = func() (bool, error) {
		t.Fatal("requestAccess should not run when legacy usage description is missing")
		return false, nil
	}

	err := checkDarwinCalendarAuthorizationRequest()
	if err == nil {
		t.Fatal("expected error when legacy calendar usage description is missing")
	}
	if !strings.Contains(err.Error(), "NSCalendarsUsageDescription") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinCalendarCheckReadAccess_RequestsFullAccessWhenStatusNotDetermined(t *testing.T) {
	restore := restoreDarwinCalendarAccessFuncs()
	defer restore()

	darwinCalendarSupportsFullAccessRequestFunc = func() bool { return true }
	darwinCalendarHasFullAccessUsageDescriptionFunc = func() bool { return true }
	darwinCalendarHasLegacyUsageDescriptionFunc = func() bool { return true }

	statusCalls := 0
	darwinCalendarAuthorizationStatusFunc = func() (int, error) {
		statusCalls++
		return darwinCalendarAuthorizationNotDetermined, nil
	}

	requested := false
	darwinCalendarRequestFullAccessFunc = func() (bool, error) {
		requested = true
		return true, nil
	}

	if err := darwinCalendarCheckReadAccess(); err != nil {
		t.Fatalf("darwinCalendarCheckReadAccess() error = %v", err)
	}
	if statusCalls == 0 {
		t.Fatal("expected authorization status to be checked")
	}
	if !requested {
		t.Fatal("expected full access request to run")
	}
}

func TestDarwinCalendarCheckReadAccess_ReturnsPermissionRequiredWhenRequestDenied(t *testing.T) {
	restore := restoreDarwinCalendarAccessFuncs()
	defer restore()

	darwinCalendarSupportsFullAccessRequestFunc = func() bool { return true }
	darwinCalendarHasFullAccessUsageDescriptionFunc = func() bool { return true }
	darwinCalendarHasLegacyUsageDescriptionFunc = func() bool { return true }
	darwinCalendarAuthorizationStatusFunc = func() (int, error) {
		return darwinCalendarAuthorizationNotDetermined, nil
	}
	darwinCalendarRequestFullAccessFunc = func() (bool, error) {
		return false, nil
	}

	err := darwinCalendarCheckReadAccess()
	if err == nil {
		t.Fatal("expected permission-required error when access request is denied")
	}
	if !strings.Contains(err.Error(), "CALENDAR_PERMISSION_REQUIRED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinCalendarCheckWriteAccess_AllowsWriteOnlyWithoutRequest(t *testing.T) {
	restore := restoreDarwinCalendarAccessFuncs()
	defer restore()

	darwinCalendarSupportsFullAccessRequestFunc = func() bool { return true }
	darwinCalendarHasFullAccessUsageDescriptionFunc = func() bool { return true }
	darwinCalendarHasLegacyUsageDescriptionFunc = func() bool { return true }
	darwinCalendarAuthorizationStatusFunc = func() (int, error) {
		return darwinCalendarAuthorizationWriteOnly, nil
	}
	darwinCalendarRequestFullAccessFunc = func() (bool, error) {
		t.Fatal("requestFullAccess should not run when write-only access already exists")
		return false, nil
	}

	if err := darwinCalendarCheckWriteAccess(); err != nil {
		t.Fatalf("darwinCalendarCheckWriteAccess() error = %v", err)
	}
}

func TestDarwinNativeCalendarStore_SmokeListToday(t *testing.T) {
	store := newDarwinNativeCalendarStoreOrNil(nil)
	if store == nil {
		t.Skip("native darwin calendar store unavailable")
	}

	start := time.Now().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	events, err := store.List(context.Background(), "", CalendarQueryOptions{
		Start: &start,
		End:   &end,
		Limit: 5,
	})
	if err != nil {
		if strings.Contains(err.Error(), "NSCalendarsFullAccessUsageDescription") ||
			strings.Contains(err.Error(), "NSCalendarsUsageDescription") {
			t.Logf("native calendar read requires bundled calendar usage description: %v", err)
			return
		}
		if strings.Contains(err.Error(), "CALENDAR_PERMISSION_REQUIRED") {
			t.Logf("native calendar read requires permission: %v", err)
			return
		}
		t.Fatalf("native calendar read failed: %v", err)
	}
	t.Logf("native calendar read ok: %d event(s) returned", len(events))
}

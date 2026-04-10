//go:build darwin

package tools

import (
	"context"
	"strings"
	"testing"
)

func restoreDarwinContactsAccessFuncs() func() {
	prevHasUsage := darwinContactsHasUsageDescriptionFunc
	prevStatus := darwinContactsAuthorizationStatusFunc
	prevRequest := darwinContactsRequestAccessFunc
	return func() {
		darwinContactsHasUsageDescriptionFunc = prevHasUsage
		darwinContactsAuthorizationStatusFunc = prevStatus
		darwinContactsRequestAccessFunc = prevRequest
	}
}

func TestCheckDarwinContactsAuthorizationRequestShortCircuitsWhenUsageMissing(t *testing.T) {
	restore := restoreDarwinContactsAccessFuncs()
	defer restore()

	darwinContactsHasUsageDescriptionFunc = func() bool { return false }
	darwinContactsAuthorizationStatusFunc = func() (int, error) {
		t.Fatal("authorization status should not run when usage description is missing")
		return 0, nil
	}
	darwinContactsRequestAccessFunc = func() (bool, error) {
		t.Fatal("requestAccess should not run when usage description is missing")
		return false, nil
	}

	err := checkDarwinContactsAuthorizationRequest()
	if err == nil {
		t.Fatal("expected error when contacts usage description is missing")
	}
	if !strings.Contains(err.Error(), "NSContactsUsageDescription") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinContactsEnsureAccess_RequestsPermissionWhenStatusNotDetermined(t *testing.T) {
	restore := restoreDarwinContactsAccessFuncs()
	defer restore()

	darwinContactsHasUsageDescriptionFunc = func() bool { return true }

	statusCalls := 0
	darwinContactsAuthorizationStatusFunc = func() (int, error) {
		statusCalls++
		return darwinContactsAuthorizationNotDetermined, nil
	}

	requested := false
	darwinContactsRequestAccessFunc = func() (bool, error) {
		requested = true
		return true, nil
	}

	if err := darwinContactsEnsureAccess(); err != nil {
		t.Fatalf("darwinContactsEnsureAccess() error = %v", err)
	}
	if statusCalls == 0 {
		t.Fatal("expected authorization status to be checked")
	}
	if !requested {
		t.Fatal("expected contacts permission request to run")
	}
}

func TestDarwinContactsEnsureAccess_ReturnsPermissionRequiredWhenRequestDenied(t *testing.T) {
	restore := restoreDarwinContactsAccessFuncs()
	defer restore()

	darwinContactsHasUsageDescriptionFunc = func() bool { return true }
	darwinContactsAuthorizationStatusFunc = func() (int, error) {
		return darwinContactsAuthorizationNotDetermined, nil
	}
	darwinContactsRequestAccessFunc = func() (bool, error) {
		return false, nil
	}

	err := darwinContactsEnsureAccess()
	if err == nil {
		t.Fatal("expected permission-required error when contacts access request is denied")
	}
	if !strings.Contains(err.Error(), "CONTACTS_PERMISSION_REQUIRED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinNativeContactsStore_SmokeList(t *testing.T) {
	store := newDarwinNativeContactsStoreOrNil(nil)
	if store == nil {
		t.Skip("native darwin contacts store unavailable")
	}

	contacts, err := store.List(context.Background(), "", ContactsQueryOptions{Limit: 5})
	if err != nil {
		if strings.Contains(err.Error(), "NSContactsUsageDescription") {
			t.Logf("native contacts read requires bundled contacts usage description: %v", err)
			return
		}
		if strings.Contains(err.Error(), "CONTACTS_PERMISSION_REQUIRED") {
			t.Logf("native contacts read requires permission: %v", err)
			return
		}
		t.Fatalf("native contacts read failed: %v", err)
	}
	t.Logf("native contacts read ok: %d contact(s) returned", len(contacts))
}

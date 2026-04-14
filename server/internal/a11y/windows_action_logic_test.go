package a11y

import (
	"errors"
	"testing"
)

func TestWindowsActionResultWithPlan_ReturnsSemanticResultOnPrimarySuccess(t *testing.T) {
	primaryCalls := 0
	fallbackCalls := 0

	result, err := windowsActionResultWithPlan(
		"windows",
		"42",
		"click",
		"",
		600,
		windowsActionPlan{
			ExecutionMode: "semantic",
			Primary:       windowsActionDefaultAction,
			Fallback:      windowsActionInputClick,
		},
		func(plan windowsActionPlan, value string) error {
			primaryCalls++
			if plan.Primary != windowsActionDefaultAction {
				t.Fatalf("plan.Primary = %q, want default_action", plan.Primary)
			}
			if value != "" {
				t.Fatalf("value = %q, want empty", value)
			}
			return nil
		},
		nil,
		func(string, string, int, bool) error {
			fallbackCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsActionResultWithPlan() error = %v", err)
	}
	if primaryCalls != 1 {
		t.Fatalf("primaryCalls = %d, want 1", primaryCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallbackCalls = %d, want 0", fallbackCalls)
	}
	if result.ExecutionMode != "semantic" {
		t.Fatalf("execution_mode = %q, want semantic", result.ExecutionMode)
	}
	if result.WindowID != "42" {
		t.Fatalf("window_id = %q, want 42", result.WindowID)
	}
}

func TestWindowsActionResultWithPlan_FallsBackToInputWhenPrimaryFails(t *testing.T) {
	primaryCalls := 0
	fallbackCalls := 0
	fallbackHoldMS := 0

	result, err := windowsActionResultWithPlan(
		"windows",
		"42",
		"type",
		"hello",
		0,
		windowsActionPlan{
			ExecutionMode: "input",
			Primary:       windowsActionSelectFocus,
			Fallback:      windowsActionInputType,
		},
		func(plan windowsActionPlan, value string) error {
			primaryCalls++
			if plan.Primary != windowsActionSelectFocus {
				t.Fatalf("plan.Primary = %q, want select_focus", plan.Primary)
			}
			if value != "hello" {
				t.Fatalf("value = %q, want hello", value)
			}
			return errors.New("accSelect failed")
		},
		nil,
		func(fallback string, value string, holdMS int, primarySucceeded bool) error {
			fallbackCalls++
			fallbackHoldMS = holdMS
			if primarySucceeded {
				t.Fatal("primarySucceeded = true, want false when primary failed")
			}
			if fallback != windowsActionInputType {
				t.Fatalf("fallback = %q, want input_type", fallback)
			}
			if value != "hello" {
				t.Fatalf("value = %q, want hello", value)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsActionResultWithPlan() error = %v", err)
	}
	if primaryCalls != 1 {
		t.Fatalf("primaryCalls = %d, want 1", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallbackCalls = %d, want 1", fallbackCalls)
	}
	if fallbackHoldMS != DefaultHoldMS {
		t.Fatalf("fallbackHoldMS = %d, want %d", fallbackHoldMS, DefaultHoldMS)
	}
	if result.ExecutionMode != "input" {
		t.Fatalf("execution_mode = %q, want input", result.ExecutionMode)
	}
}

func TestWindowsActionResultWithPlan_TypeContinuesToFallbackAfterPrimaryFocusSucceeds(t *testing.T) {
	primaryCalls := 0
	fallbackCalls := 0

	result, err := windowsActionResultWithPlan(
		"windows",
		"42",
		"type",
		"hello",
		0,
		windowsActionPlan{
			ExecutionMode: "input",
			Primary:       windowsActionSelectFocus,
			Fallback:      windowsActionInputType,
		},
		func(plan windowsActionPlan, value string) error {
			primaryCalls++
			if plan.Primary != windowsActionSelectFocus {
				t.Fatalf("plan.Primary = %q, want select_focus", plan.Primary)
			}
			if value != "hello" {
				t.Fatalf("value = %q, want hello", value)
			}
			return nil
		},
		nil,
		func(fallback string, value string, holdMS int, primarySucceeded bool) error {
			fallbackCalls++
			if !primarySucceeded {
				t.Fatal("primarySucceeded = false, want true")
			}
			if fallback != windowsActionInputType {
				t.Fatalf("fallback = %q, want input_type", fallback)
			}
			if value != "hello" {
				t.Fatalf("value = %q, want hello", value)
			}
			if holdMS != DefaultHoldMS {
				t.Fatalf("holdMS = %d, want %d", holdMS, DefaultHoldMS)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsActionResultWithPlan() error = %v", err)
	}
	if primaryCalls != 1 {
		t.Fatalf("primaryCalls = %d, want 1", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallbackCalls = %d, want 1", fallbackCalls)
	}
	if result.ExecutionMode != "input" {
		t.Fatalf("execution_mode = %q, want input", result.ExecutionMode)
	}
}

func TestWindowsActionResultWithPlan_TypeFallsBackWhenSemanticPutValueIsNotConfirmed(t *testing.T) {
	primaryCalls := 0
	verifyCalls := 0
	fallbackCalls := 0

	result, err := windowsActionResultWithPlan(
		"windows",
		"42",
		"type",
		"hello",
		0,
		windowsActionPlan{
			ExecutionMode: "semantic",
			Primary:       windowsActionPutValue,
			Fallback:      windowsActionInputType,
		},
		func(plan windowsActionPlan, value string) error {
			primaryCalls++
			if plan.Primary != windowsActionPutValue {
				t.Fatalf("plan.Primary = %q, want put_value", plan.Primary)
			}
			if value != "hello" {
				t.Fatalf("value = %q, want hello", value)
			}
			return nil
		},
		func(plan windowsActionPlan, value string) bool {
			verifyCalls++
			if plan.Primary != windowsActionPutValue {
				t.Fatalf("verify plan.Primary = %q, want put_value", plan.Primary)
			}
			if value != "hello" {
				t.Fatalf("verify value = %q, want hello", value)
			}
			return false
		},
		func(fallback string, value string, holdMS int, primarySucceeded bool) error {
			fallbackCalls++
			if fallback != windowsActionInputType {
				t.Fatalf("fallback = %q, want input_type", fallback)
			}
			if primarySucceeded {
				t.Fatal("primarySucceeded = true, want false when semantic write was not confirmed")
			}
			if holdMS != DefaultHoldMS {
				t.Fatalf("holdMS = %d, want %d", holdMS, DefaultHoldMS)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsActionResultWithPlan() error = %v", err)
	}
	if primaryCalls != 1 {
		t.Fatalf("primaryCalls = %d, want 1", primaryCalls)
	}
	if verifyCalls != 1 {
		t.Fatalf("verifyCalls = %d, want 1", verifyCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallbackCalls = %d, want 1", fallbackCalls)
	}
	if result.ExecutionMode != "input" {
		t.Fatalf("execution_mode = %q, want input", result.ExecutionMode)
	}
}

func TestWindowsActionResultWithPlan_ReturnsUnsupportedForUnsupportedPlan(t *testing.T) {
	_, err := windowsActionResultWithPlan(
		"windows",
		"42",
		"toggle",
		"",
		600,
		windowsActionPlan{
			Unsupported:       true,
			UnsupportedReason: "element does not support toggle",
		},
		nil,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("windowsActionResultWithPlan() error = nil, want unsupported_action")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_action" {
		t.Fatalf("code = %q, want unsupported_action", runtimeErr.Code)
	}
}

func TestWindowsActionResultWithPlan_ReturnsUnsupportedWhenNoFallbackExists(t *testing.T) {
	_, err := windowsActionResultWithPlan(
		"windows",
		"42",
		"click",
		"",
		600,
		windowsActionPlan{
			ExecutionMode: "semantic",
			Primary:       windowsActionDefaultAction,
		},
		func(windowsActionPlan, string) error {
			return errors.New("accDoDefaultAction failed")
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("windowsActionResultWithPlan() error = nil, want unsupported_action")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_action" {
		t.Fatalf("code = %q, want unsupported_action", runtimeErr.Code)
	}
}

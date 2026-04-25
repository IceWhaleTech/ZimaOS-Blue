package a11y

import "testing"

func TestWindowsScrollResultWithInput_NormalizesDirectionAndLines(t *testing.T) {
	bringFrontCalls := 0
	sendCalls := 0
	gotHorizontal := false
	gotDelta := int32(0)

	result, err := windowsScrollResultWithInput(
		"windows",
		"42",
		"",
		0,
		func() {
			bringFrontCalls++
		},
		func(horizontal bool, delta int32) error {
			sendCalls++
			gotHorizontal = horizontal
			gotDelta = delta
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsScrollResultWithInput() error = %v", err)
	}
	if bringFrontCalls != 1 {
		t.Fatalf("bringFrontCalls = %d, want 1", bringFrontCalls)
	}
	if sendCalls != 1 {
		t.Fatalf("sendCalls = %d, want 1", sendCalls)
	}
	if gotHorizontal {
		t.Fatal("gotHorizontal = true, want false for default vertical scroll")
	}
	if gotDelta != -720 {
		t.Fatalf("gotDelta = %d, want -720", gotDelta)
	}
	if result.WindowID != "42" || result.ExecutionMode != "input" {
		t.Fatalf("result = %+v, want input result for window 42", result)
	}
}

func TestWindowsScrollResultWithInput_UsesHorizontalDeltaForLeft(t *testing.T) {
	gotHorizontal := false
	gotDelta := int32(0)

	_, err := windowsScrollResultWithInput(
		"windows",
		"42",
		"left",
		3,
		nil,
		func(horizontal bool, delta int32) error {
			gotHorizontal = horizontal
			gotDelta = delta
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsScrollResultWithInput() error = %v", err)
	}
	if !gotHorizontal {
		t.Fatal("gotHorizontal = false, want true for horizontal scroll")
	}
	if gotDelta != -360 {
		t.Fatalf("gotDelta = %d, want -360", gotDelta)
	}
}

func TestWindowsPointerMoveResult_DelegatesToMover(t *testing.T) {
	moveCalls := 0

	result, err := windowsPointerMoveResult(
		"windows",
		120,
		340,
		func(x int, y int) error {
			moveCalls++
			if x != 120 || y != 340 {
				t.Fatalf("move args = (%d,%d), want (120,340)", x, y)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsPointerMoveResult() error = %v", err)
	}
	if moveCalls != 1 {
		t.Fatalf("moveCalls = %d, want 1", moveCalls)
	}
	if result.ExecutionMode != "input" || result.Message != "Pointer moved" {
		t.Fatalf("result = %+v, want input pointer result", result)
	}
}

func TestWindowsKeyResult_NormalizesHoldMSAndDelegates(t *testing.T) {
	bringFrontCalls := 0
	sendCalls := 0
	gotHoldMS := 0

	result, err := windowsKeyResultWithInput(
		"windows",
		"77",
		[]string{"ctrl", "v"},
		0,
		func() {
			bringFrontCalls++
		},
		func(keys []string, holdMS int) error {
			sendCalls++
			gotHoldMS = holdMS
			if len(keys) != 2 || keys[0] != "ctrl" || keys[1] != "v" {
				t.Fatalf("keys = %#v, want [ctrl v]", keys)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsKeyResultWithInput() error = %v", err)
	}
	if bringFrontCalls != 1 {
		t.Fatalf("bringFrontCalls = %d, want 1", bringFrontCalls)
	}
	if sendCalls != 1 {
		t.Fatalf("sendCalls = %d, want 1", sendCalls)
	}
	if gotHoldMS != DefaultHoldMS {
		t.Fatalf("gotHoldMS = %d, want %d", gotHoldMS, DefaultHoldMS)
	}
	if result.WindowID != "77" || result.Message != "Keys sent" {
		t.Fatalf("result = %+v, want keys result for window 77", result)
	}
}

func TestWindowsFocusedTextResultWithInput_UsesClipboardFallback(t *testing.T) {
	calls := 0
	result, err := windowsFocusedTextResultWithInput("windows", "42", "hello", func(value string) (string, error) {
		calls++
		if value != "hello" {
			t.Fatalf("value = %q, want hello", value)
		}
		return "clipboard", nil
	})
	if err != nil {
		t.Fatalf("windowsFocusedTextResultWithInput() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if result.HostOS != "windows" || result.WindowID != "42" {
		t.Fatalf("result target = %s/%s, want windows/42", result.HostOS, result.WindowID)
	}
	if result.InputMethod != "clipboard" || result.VerificationMethod != "focused_text" || !result.TargetHit || !result.VerificationPassed {
		t.Fatalf("result = %#v, want verified clipboard focused_text", result)
	}
}

//go:build darwin

package speech

import (
	"strings"
	"testing"
)

func restoreSTTPreflightFuncs() func() {
	prevSpeech := hasSpeechUsageDescriptionFunc
	prevMic := hasMicrophoneUsageDescriptionFunc
	prevSign := verifyCodeSignatureFunc
	return func() {
		hasSpeechUsageDescriptionFunc = prevSpeech
		hasMicrophoneUsageDescriptionFunc = prevMic
		verifyCodeSignatureFunc = prevSign
	}
}

func TestCheckSpeechRecognitionAuthorizationRequestShortCircuitsWhenUsageMissing(t *testing.T) {
	restore := restoreSTTPreflightFuncs()
	defer restore()

	hasSpeechUsageDescriptionFunc = func() bool { return false }
	verifyCodeSignatureFunc = func() bool {
		t.Fatal("verifyCodeSignature should not run when usage description is missing")
		return false
	}

	err := checkSpeechRecognitionAuthorizationRequest()
	if err == nil {
		t.Fatal("expected error when speech usage description is missing")
	}
	if !strings.Contains(err.Error(), "NSSpeechRecognitionUsageDescription") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckSpeechRecognitionAuthorizationRequestRequiresValidSignature(t *testing.T) {
	restore := restoreSTTPreflightFuncs()
	defer restore()

	hasSpeechUsageDescriptionFunc = func() bool { return true }
	calledVerify := false
	verifyCodeSignatureFunc = func() bool {
		calledVerify = true
		return false
	}

	err := checkSpeechRecognitionAuthorizationRequest()
	if err == nil {
		t.Fatal("expected error when code signature is invalid")
	}
	if !calledVerify {
		t.Fatal("expected code signature verification to run")
	}
	if !strings.Contains(err.Error(), "code signature") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckMicrophoneAccessRequiresUsageDescription(t *testing.T) {
	restore := restoreSTTPreflightFuncs()
	defer restore()

	hasMicrophoneUsageDescriptionFunc = func() bool { return false }

	err := CheckMicrophoneAccess()
	if err == nil {
		t.Fatal("expected error when microphone usage description is missing")
	}
	if !strings.Contains(err.Error(), "NSMicrophoneUsageDescription") {
		t.Fatalf("unexpected error: %v", err)
	}
}

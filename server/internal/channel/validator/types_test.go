package validator

import (
	"context"
	"testing"
	"time"
)

func TestResult_Success(t *testing.T) {
	result := NewSuccessResult("testSuccess", map[string]interface{}{
		"bot_name": "TestBot",
		"bot_id":   "180",
	})

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["bot_name"] != "TestBot" {
		t.Errorf("expected bot_name 'TestBot', got '%v'", result.Data["bot_name"])
	}
	if result.Error != "" {
		t.Errorf("expected empty Error, got '%s'", result.Error)
	}
}

func TestResult_Error(t *testing.T) {
	result := NewErrorResult("invalidToken", "The provided token is invalid")

	if result.Success {
		t.Error("expected Success to be false")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
	if result.Error != "The provided token is invalid" {
		t.Errorf("expected specific error message, got '%s'", result.Error)
	}
}

func TestResult_MissingField(t *testing.T) {
	result := NewMissingFieldResult("botToken")

	if result.Success {
		t.Error("expected Success to be false")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestValidatorFunc(t *testing.T) {
	// Test that ValidatorFunc implements Validator interface
	var v Validator = ValidatorFunc(func(ctx context.Context, config map[string]string) Result {
		if config["token"] == "valid" {
			return NewSuccessResult("testSuccess", nil)
		}
		return NewErrorResult("invalidToken", "invalid token")
	})

	// Test with valid config
	result := v.Validate(context.Background(), map[string]string{"token": "valid"})
	if !result.Success {
		t.Error("expected success with valid token")
	}

	// Test with invalid config
	result = v.Validate(context.Background(), map[string]string{"token": "invalid"})
	if result.Success {
		t.Error("expected failure with invalid token")
	}
}

func TestValidatorTimeout(t *testing.T) {
	// Test that validators respect context timeout
	slowValidator := ValidatorFunc(func(ctx context.Context, config map[string]string) Result {
		select {
		case <-ctx.Done():
			return NewErrorResult("timeout", "connection timed out")
		case <-time.After(5 * time.Second):
			return NewSuccessResult("testSuccess", nil)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := slowValidator.Validate(ctx, nil)
	if result.Success {
		t.Error("expected timeout error")
	}
	if result.MessageKey != "timeout" {
		t.Errorf("expected MessageKey 'timeout', got '%s'", result.MessageKey)
	}
}

func TestDefaultTimeout(t *testing.T) {
	if DefaultTimeout != 10*time.Second {
		t.Errorf("expected DefaultTimeout to be 10s, got %v", DefaultTimeout)
	}
}

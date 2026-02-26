package tools

import (
	"context"
	"testing"
)

func TestWithSessionID(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionID(ctx, "conv-123")

	got := GetSessionID(ctx)
	if got != "conv-123" {
		t.Errorf("GetSessionID() = %q, want %q", got, "conv-123")
	}
}

func TestGetSessionID_Empty(t *testing.T) {
	ctx := context.Background()
	got := GetSessionID(ctx)
	if got != "" {
		t.Errorf("GetSessionID() on empty ctx = %q, want empty", got)
	}
}

func TestSessionIDDoesNotInterfereWithOtherKeys(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionID(ctx, "sess-1")
	ctx = WithUserID(ctx, "user-1")
	ctx = WithLang(ctx, "zh-CN")

	if got := GetSessionID(ctx); got != "sess-1" {
		t.Errorf("GetSessionID() = %q, want %q", got, "sess-1")
	}
	if got := GetUserID(ctx); got != "user-1" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-1")
	}
	if got := GetLang(ctx); got != "zh-CN" {
		t.Errorf("GetLang() = %q, want %q", got, "zh-CN")
	}
}

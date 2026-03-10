package tools

import (
	"context"
	"testing"
)

func TestEmitBrowserProgressIncludesRecipeName(t *testing.T) {
	var emitted map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		emitted = card
	})

	emitBrowserProgress(ctx, "recipe", "Running login recipe", "running", "", map[string]interface{}{"recipe_name": "login recipe"})

	if emitted == nil {
		t.Fatal("expected browser progress card to be emitted")
	}
	if got := emitted["type"]; got != "browser-progress" {
		t.Fatalf("type = %v, want browser-progress", got)
	}
	if got := emitted["recipe_name"]; got != "login recipe" {
		t.Fatalf("recipe_name = %v, want login recipe", got)
	}
	if got := emitted["typeless"]; got != true {
		t.Fatalf("typeless = %v, want true", got)
	}
}

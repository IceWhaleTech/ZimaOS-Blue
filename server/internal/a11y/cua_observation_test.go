package a11y

import (
	"strings"
	"testing"
)

func TestBuildCUAObservationCompactsInteractiveSnapshotForModel(t *testing.T) {
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{WindowID: "win-1", Title: "Feishu", Mode: "ax"}, &Node{
		Role: "window",
		Name: "Feishu",
		Children: []*Node{
			{Token: "composer", Role: "text_field", Name: "Message", Interactive: true, ValueSettable: true, Bounds: NormalizedRect{X: 0.1, Y: 0.8, Width: 0.8, Height: 0.1}},
			{Token: "send", Role: "button", Name: "Send", Interactive: true, Actions: []string{"AXPress"}, Bounds: NormalizedRect{X: 0.9, Y: 0.84, Width: 0.08, Height: 0.05}},
		},
	})

	observation := BuildCUAObservation(CUAObservationInput{
		Snapshot:         snapshot,
		ScreenshotPath:   "/tmp/feishu.png",
		LastActionResult: ActionResult{Message: "typed", VerificationPassed: true},
	})

	if observation.WindowID != "win-1" || observation.Title != "Feishu" {
		t.Fatalf("observation window = %q/%q, want win-1/Feishu", observation.WindowID, observation.Title)
	}
	if observation.ScreenshotPath != "/tmp/feishu.png" {
		t.Fatalf("ScreenshotPath = %q", observation.ScreenshotPath)
	}
	if len(observation.Elements) != 2 {
		t.Fatalf("Elements len = %d, want 2", len(observation.Elements))
	}
	if observation.Elements[0].Name != "Message" || observation.Elements[0].Role == "" || len(observation.Elements[0].Capabilities) == 0 {
		t.Fatalf("first element = %#v, want compact actionable Message element", observation.Elements[0])
	}
	if observation.LastAction.Message != "typed" || !observation.LastAction.VerificationPassed {
		t.Fatalf("LastAction = %#v", observation.LastAction)
	}
	if text := observation.ModelText(); !strings.Contains(text, "Feishu") || !strings.Contains(text, "Message") || !strings.Contains(text, "/tmp/feishu.png") {
		t.Fatalf("ModelText() = %q, want title, element and screenshot path", text)
	}
}

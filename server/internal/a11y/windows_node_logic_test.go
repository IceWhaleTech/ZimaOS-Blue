package a11y

import "testing"

func TestWindowsLikelyValueWritable_DetectsEditableRoles(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{role: "editable text", want: true},
		{role: "combo box", want: true},
		{role: "document", want: true},
		{role: "push button", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.role, func(t *testing.T) {
			if got := windowsLikelyValueWritable(tc.role); got != tc.want {
				t.Fatalf("windowsLikelyValueWritable(%q) = %v, want %v", tc.role, got, tc.want)
			}
		})
	}
}

func TestWindowsExpandedStateFromText_ParsesKnownStates(t *testing.T) {
	expanded, ok := windowsExpandedStateFromText("expanded, focusable")
	if !ok || !expanded {
		t.Fatalf("expanded state = (%v,%v), want (true,true)", expanded, ok)
	}

	expanded, ok = windowsExpandedStateFromText("collapsed")
	if !ok || expanded {
		t.Fatalf("collapsed state = (%v,%v), want (false,true)", expanded, ok)
	}

	expanded, ok = windowsExpandedStateFromText("focusable")
	if ok {
		t.Fatalf("plain state ok = %v, want false", ok)
	}
	if expanded {
		t.Fatalf("plain state expanded = %v, want false", expanded)
	}
}

func TestWindowsActionMetadataFromFields_PopulatesExpandedAndBounds(t *testing.T) {
	meta := windowsActionMetadataFromFields("editable text", "expanded", "press", true, true)
	if meta.Role != "editable text" {
		t.Fatalf("meta.Role = %q, want editable text", meta.Role)
	}
	if meta.DefaultAction != "press" {
		t.Fatalf("meta.DefaultAction = %q, want press", meta.DefaultAction)
	}
	if meta.State != "expanded" {
		t.Fatalf("meta.State = %q, want expanded", meta.State)
	}
	if !meta.HasBounds {
		t.Fatal("meta.HasBounds = false, want true")
	}
	if !meta.ValueWritable {
		t.Fatal("meta.ValueWritable = false, want true")
	}
	if meta.Expanded == nil || !*meta.Expanded {
		t.Fatalf("meta.Expanded = %#v, want true", meta.Expanded)
	}
}

func TestWindowsNodeInteractive_DetectsInteractiveSignals(t *testing.T) {
	tests := []struct {
		name string
		meta windowsActionMetadata
		want bool
	}{
		{name: "writable", meta: windowsActionMetadata{ValueWritable: true}, want: true},
		{name: "interactive role", meta: windowsActionMetadata{Role: "push button"}, want: true},
		{name: "default action", meta: windowsActionMetadata{DefaultAction: "press"}, want: true},
		{name: "plain text role", meta: windowsActionMetadata{Role: "text"}, want: false},
		{name: "plain text", meta: windowsActionMetadata{Role: "static"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := windowsNodeInteractive(tc.meta); got != tc.want {
				t.Fatalf("windowsNodeInteractive(%+v) = %v, want %v", tc.meta, got, tc.want)
			}
		})
	}
}

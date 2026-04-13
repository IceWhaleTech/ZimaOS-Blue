//go:build darwin

package a11y

import "testing"

func TestNormalizeDarwinRole(t *testing.T) {
	cases := map[string]string{
		"AXButton":             "button",
		"AXTextField":          "text_field",
		"AXScrollArea":         "scroll_area",
		"AXDisclosureTriangle": "disclosure_triangle",
		"":                     "element",
	}

	for input, expected := range cases {
		if got := normalizeDarwinRole(input); got != expected {
			t.Fatalf("normalizeDarwinRole(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestDarwinKeyCodeForName(t *testing.T) {
	cases := map[string]uint16{
		"return": 0x24,
		"enter":  0x4C,
		"space":  0x31,
		"tab":    0x30,
		"escape": 0x35,
		"left":   0x7B,
		"right":  0x7C,
		"up":     0x7E,
		"down":   0x7D,
	}

	for input, expected := range cases {
		got, ok := darwinKeyCodeForName(input)
		if !ok {
			t.Fatalf("darwinKeyCodeForName(%q) not found", input)
		}
		if got != expected {
			t.Fatalf("darwinKeyCodeForName(%q) = 0x%X, want 0x%X", input, got, expected)
		}
	}
}

func TestDarwinModifierFlags(t *testing.T) {
	flags := darwinModifierFlags([]string{"cmd", "shift", "p"})
	if flags&darwinEventFlagCommand == 0 {
		t.Fatalf("flags = 0x%X, want command bit", flags)
	}
	if flags&darwinEventFlagShift == 0 {
		t.Fatalf("flags = 0x%X, want shift bit", flags)
	}
	if flags&darwinEventFlagAlternate != 0 {
		t.Fatalf("flags = 0x%X, did not expect option bit", flags)
	}
}

func TestDarwinIncludeWindowRecord(t *testing.T) {
	regular := darwinWindowRecord{
		ID:      "1",
		AppName: "Feishu",
		PID:     99,
		Layer:   0,
		Bounds:  darwinRect{Size: darwinSize{Width: 1200, Height: 800}},
	}
	if !darwinIncludeWindowRecord(regular) {
		t.Fatal("expected regular app window to be included")
	}

	systemOverlay := regular
	systemOverlay.AppName = "控制中心"
	systemOverlay.Layer = 25
	if darwinIncludeWindowRecord(systemOverlay) {
		t.Fatal("expected system overlay window to be excluded")
	}

	dock := regular
	dock.AppName = "Dock"
	if darwinIncludeWindowRecord(dock) {
		t.Fatal("expected Dock window to be excluded")
	}

	tooSmall := regular
	tooSmall.Bounds = darwinRect{Size: darwinSize{Width: 1, Height: 0}}
	if darwinIncludeWindowRecord(tooSmall) {
		t.Fatal("expected tiny window to be excluded")
	}
}

func TestDarwinMarkFocusedRecord(t *testing.T) {
	records := []darwinWindowRecord{
		{ID: "1", AppName: "Code", PID: 10, Title: "Editor", Bounds: darwinRect{Size: darwinSize{Width: 1000, Height: 700}}},
		{ID: "2", AppName: "Feishu", PID: 20, Title: "飞书", Bounds: darwinRect{Origin: darwinPoint{X: 50, Y: 50}, Size: darwinSize{Width: 1200, Height: 800}}},
	}
	darwinMarkFocusedRecord(records, 20, "飞书", darwinRect{Origin: darwinPoint{X: 50, Y: 50}, Size: darwinSize{Width: 1200, Height: 800}})
	if records[0].Focused {
		t.Fatal("expected first record not to be focused")
	}
	if !records[1].Focused {
		t.Fatal("expected second record to be focused")
	}

	darwinMarkFocusedRecord(records, 0, "", darwinRect{})
	if !records[0].Focused {
		t.Fatal("expected first record to become focused fallback")
	}
}

package a11y

import "testing"

func TestNormalizeHoldMS(t *testing.T) {
	if got := normalizeHoldMS(0); got != 600 {
		t.Fatalf("normalizeHoldMS(0) = %d, want 600", got)
	}
	if got := normalizeHoldMS(-50); got != 600 {
		t.Fatalf("normalizeHoldMS(-50) = %d, want 600", got)
	}
	if got := normalizeHoldMS(250); got != 250 {
		t.Fatalf("normalizeHoldMS(250) = %d, want 250", got)
	}
}

func TestPlanDarwinAction(t *testing.T) {
	expanded := true
	cases := []struct {
		name  string
		act   string
		meta  darwinActionMetadata
		check func(t *testing.T, plan darwinActionPlan)
	}{
		{
			name: "click prefers press semantics",
			act:  "click",
			meta: darwinActionMetadata{
				Role:             "button",
				DefaultAction:    "press",
				AvailableActions: []string{"AXPress"},
			},
			check: func(t *testing.T, plan darwinActionPlan) {
				if plan.SemanticAction != "AXPress" || plan.ExecutionMode != "semantic" {
					t.Fatalf("plan = %#v, want semantic AXPress", plan)
				}
			},
		},
		{
			name: "type uses set value when allowed",
			act:  "type",
			meta: darwinActionMetadata{
				Role:          "text_field",
				ValueSettable: true,
			},
			check: func(t *testing.T, plan darwinActionPlan) {
				if !plan.SetValue || plan.ExecutionMode != "semantic" {
					t.Fatalf("plan = %#v, want semantic set-value plan", plan)
				}
			},
		},
		{
			name: "expand rejects already expanded element",
			act:  "expand",
			meta: darwinActionMetadata{
				Role:             "disclosure_triangle",
				Expanded:         &expanded,
				AvailableActions: []string{"AXPress"},
			},
			check: func(t *testing.T, plan darwinActionPlan) {
				if !plan.Unsupported || plan.UnsupportedReason == "" {
					t.Fatalf("plan = %#v, want unsupported expanded-state rejection", plan)
				}
			},
		},
		{
			name: "long press falls back to pointer input",
			act:  "long_press",
			meta: darwinActionMetadata{
				Role: "button",
			},
			check: func(t *testing.T, plan darwinActionPlan) {
				if plan.InputFallback != darwinInputFallbackClickHold || plan.ExecutionMode != "input" {
					t.Fatalf("plan = %#v, want input hold fallback", plan)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, planDarwinAction(tc.act, tc.meta))
		})
	}
}

func TestPlanWindowsAction(t *testing.T) {
	expanded := false
	cases := []struct {
		name  string
		act   string
		meta  windowsActionMetadata
		check func(t *testing.T, plan windowsActionPlan)
	}{
		{
			name: "right click stays input only",
			act:  "right_click",
			meta: windowsActionMetadata{
				HasBounds: true,
			},
			check: func(t *testing.T, plan windowsActionPlan) {
				if plan.Fallback != windowsActionInputRightClick || plan.ExecutionMode != "input" {
					t.Fatalf("plan = %#v, want input right-click fallback", plan)
				}
			},
		},
		{
			name: "click defaults to msaa then input fallback",
			act:  "click",
			meta: windowsActionMetadata{
				Role:          "push button",
				DefaultAction: "Press",
				HasBounds:     true,
			},
			check: func(t *testing.T, plan windowsActionPlan) {
				if plan.Primary != windowsActionDefaultAction || plan.Fallback != windowsActionInputClick {
					t.Fatalf("plan = %#v, want default-action then click fallback", plan)
				}
			},
		},
		{
			name: "focus prefers take focus selection",
			act:  "focus",
			meta: windowsActionMetadata{
				Role: "editable text",
			},
			check: func(t *testing.T, plan windowsActionPlan) {
				if plan.Primary != windowsActionSelectFocus || plan.SelectFlags != windowsSELFLAGTakeFocus {
					t.Fatalf("plan = %#v, want take-focus selection", plan)
				}
			},
		},
		{
			name: "type uses put value when writable",
			act:  "type",
			meta: windowsActionMetadata{
				Role:          "editable text",
				ValueWritable: true,
			},
			check: func(t *testing.T, plan windowsActionPlan) {
				if plan.Primary != windowsActionPutValue || plan.ExecutionMode != "semantic" {
					t.Fatalf("plan = %#v, want semantic put-value", plan)
				}
			},
		},
		{
			name: "expand requires state signal",
			act:  "expand",
			meta: windowsActionMetadata{
				Role:          "outline button",
				DefaultAction: "Press",
				Expanded:      &expanded,
			},
			check: func(t *testing.T, plan windowsActionPlan) {
				if plan.Primary != windowsActionDefaultAction || plan.Unsupported {
					t.Fatalf("plan = %#v, want expandable default action", plan)
				}
			},
		},
		{
			name: "select stays unsupported without selection support",
			act:  "select",
			meta: windowsActionMetadata{
				Role: "pane",
			},
			check: func(t *testing.T, plan windowsActionPlan) {
				if !plan.Unsupported {
					t.Fatalf("plan = %#v, want unsupported selection", plan)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, planWindowsAction(tc.act, tc.meta))
		})
	}
}

package a11y

import (
	"fmt"
	"strings"
)

const defaultCUAObservationElementLimit = 48

type CUAObservationInput struct {
	Snapshot         *Snapshot
	ScreenshotPath   string
	LastActionResult ActionResult
	LastError        string
	ElementLimit     int
}

type CUAObservation struct {
	WindowID       string                  `json:"window_id,omitempty"`
	Title          string                  `json:"title,omitempty"`
	Mode           string                  `json:"mode,omitempty"`
	ScreenshotPath string                  `json:"screenshot_path,omitempty"`
	Elements       []CUAObservationElement `json:"elements,omitempty"`
	LastAction     ActionResult            `json:"last_action,omitempty"`
	LastError      string                  `json:"last_error,omitempty"`
}

type CUAObservationElement struct {
	ID           string         `json:"id,omitempty"`
	Role         string         `json:"role,omitempty"`
	Name         string         `json:"name,omitempty"`
	Value        string         `json:"value,omitempty"`
	Path         string         `json:"path,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Bounds       NormalizedRect `json:"bounds,omitempty"`
	Focused      bool           `json:"focused,omitempty"`
	Enabled      bool           `json:"enabled,omitempty"`
}

func BuildCUAObservation(input CUAObservationInput) CUAObservation {
	snapshot := input.Snapshot
	observation := CUAObservation{
		ScreenshotPath: strings.TrimSpace(input.ScreenshotPath),
		LastAction:     input.LastActionResult,
		LastError:      strings.TrimSpace(input.LastError),
	}
	if snapshot == nil {
		return observation
	}
	observation.WindowID = strings.TrimSpace(snapshot.WindowID)
	observation.Title = strings.TrimSpace(snapshot.Title)
	observation.Mode = strings.TrimSpace(snapshot.Mode)
	limit := input.ElementLimit
	if limit <= 0 {
		limit = defaultCUAObservationElementLimit
	}
	for _, node := range snapshot.Nodes {
		if len(observation.Elements) >= limit {
			break
		}
		if !node.referenceable() {
			continue
		}
		capabilities := append([]string(nil), node.Capabilities...)
		if len(capabilities) == 0 {
			capabilities = cuaObservationFallbackCapabilities(node)
		}
		observation.Elements = append(observation.Elements, CUAObservationElement{
			ID:           strings.TrimSpace(valueOrDefaultString(node.StableID, node.BackendToken)),
			Role:         strings.TrimSpace(node.Role),
			Name:         strings.TrimSpace(node.Name),
			Value:        strings.TrimSpace(node.Value),
			Path:         strings.TrimSpace(node.Path),
			Capabilities: capabilities,
			Bounds:       node.Bounds,
			Focused:      node.Focused,
			Enabled:      node.Enabled,
		})
	}
	return observation
}

func cuaObservationFallbackCapabilities(node FlatNode) []string {
	role := normalizeSnapshotMatchValue(node.Role)
	defaultAction := normalizeSnapshotMatchValue(node.DefaultAction)
	capabilities := make([]string, 0, 2)
	if snapshotContainsAny(role, "text_field", "text_area", "editable_text", "document", "editor") {
		capabilities = append(capabilities, "focus", "set_value")
	}
	if snapshotContainsAny(role, "button", "link", "menu_item") || snapshotContainsAny(defaultAction, "press", "confirm") {
		capabilities = append(capabilities, "press")
	}
	if len(capabilities) == 0 && node.referenceable() {
		capabilities = append(capabilities, "press")
	}
	return capabilities
}

func (o CUAObservation) ModelText() string {
	var b strings.Builder
	if o.Title != "" || o.WindowID != "" {
		fmt.Fprintf(&b, "Window: %s (%s)\n", o.Title, o.WindowID)
	}
	if o.ScreenshotPath != "" {
		fmt.Fprintf(&b, "Screenshot: %s\n", o.ScreenshotPath)
	}
	if o.LastAction.Message != "" {
		fmt.Fprintf(&b, "Last action: %s verification=%t\n", o.LastAction.Message, o.LastAction.VerificationPassed)
	}
	if o.LastError != "" {
		fmt.Fprintf(&b, "Last error: %s\n", o.LastError)
	}
	for idx, element := range o.Elements {
		label := strings.TrimSpace(element.Name)
		if label == "" {
			label = strings.TrimSpace(element.Value)
		}
		fmt.Fprintf(&b, "[%d] %s %q caps=%s bounds=%.3f,%.3f,%.3f,%.3f\n", idx+1, element.Role, label, strings.Join(element.Capabilities, ","), element.Bounds.X, element.Bounds.Y, element.Bounds.Width, element.Bounds.Height)
	}
	return strings.TrimSpace(b.String())
}

func valueOrDefaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

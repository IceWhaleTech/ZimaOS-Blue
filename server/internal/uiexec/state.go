package uiexec

import "strings"

// StateFromUI extracts a lightweight, app-agnostic state abstraction from the current UI.
// pageHintOverride can be used when the caller has a better hint than the snapshot title.
func StateFromUI(ui *SemanticUI, pageHintOverride string) State {
	pageHint := strings.TrimSpace(pageHintOverride)
	if pageHint == "" && ui != nil && ui.Snapshot != nil {
		pageHint = strings.TrimSpace(ui.Snapshot.Title)
	}

	focusedRole := ""
	visible := make([]string, 0, 12)
	seen := make(map[string]struct{}, 16)
	if ui != nil {
		for _, n := range ui.Nodes {
			if focusedRole == "" && n.Focused && strings.TrimSpace(n.Role) != "" {
				focusedRole = strings.TrimSpace(n.Role)
			}
			if !n.Visible || strings.TrimSpace(n.Name) == "" {
				continue
			}
			key := normalizeText(n.Name)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			visible = append(visible, n.Name)
			if len(visible) >= 12 {
				break
			}
		}
	}

	return State{
		PageHint:        pageHint,
		VisibleEntities: visible,
		FocusedRole:     strings.TrimSpace(strings.ToLower(focusedRole)),
	}
}

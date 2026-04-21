package uiexec

import "strings"

// TryDirectAction implements opportunistic execution:
//  1. search for the target in the current UI
//  2. if found and directly operable, execute immediately
//  3. otherwise return false so the caller can fallback (scroll/search/LLM)
//
// intent examples:
// - "click"
// - "focus"
// - "type:hello"
// - "invoke:press"
func TryDirectAction(finder *Finder, exec func(Action) error, target string, intent string) bool {
	if finder == nil || exec == nil {
		return false
	}
	target = strings.TrimSpace(target)
	intent = strings.TrimSpace(strings.ToLower(intent))
	if target == "" || intent == "" {
		return false
	}

	actType := intent
	value := ""
	if idx := strings.IndexAny(intent, ":="); idx >= 0 {
		actType = strings.TrimSpace(intent[:idx])
		value = strings.TrimSpace(intent[idx+1:])
	}

	required := ""
	switch actType {
	case "click", "focus", "type", "invoke":
		required = actType
	default:
		return false
	}

	node := finder.FindNode(Query{Name: target, RequireCapability: required})
	if node == nil {
		node = finder.FindNode(Query{NameApprox: target, RequireCapability: required})
	}
	if node == nil {
		return false
	}
	if err := exec(Action{Type: actType, Target: node.ID, Value: value}); err != nil {
		return false
	}
	return true
}

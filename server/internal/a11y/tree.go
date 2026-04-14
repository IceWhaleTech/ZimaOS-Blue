package a11y

import (
	"fmt"
	"sort"
	"strings"
)

const interactiveSnapshotMaxRefs = 48

type Node struct {
	Token         string
	Role          string
	Name          string
	Value         string
	Description   string
	DefaultAction string
	Interactive   bool
	Children      []*Node
}

func BuildSnapshotTree(root *Node, interactiveOnly bool) (string, map[int]string) {
	if root == nil {
		return "", nil
	}
	lines := make([]string, 0, 32)
	refMap := make(map[int]string)
	ref := 1
	selection := buildInteractiveSnapshotSelection(root, interactiveOnly)
	if interactiveOnly && selection != nil {
		buildCompactInteractiveSnapshotLines(selection.ordered, &ref, refMap, &lines)
	} else {
		buildSnapshotTreeLines(root, interactiveOnly, nil, 0, &ref, refMap, &lines)
	}
	if len(lines) == 0 {
		return "", nil
	}
	if len(refMap) == 0 {
		refMap = nil
	}
	return strings.Join(lines, "\n"), refMap
}

func buildSnapshotTreeLines(node *Node, interactiveOnly bool, selected map[*Node]bool, depth int, nextRef *int, refMap map[int]string, lines *[]string) {
	if node == nil {
		return
	}
	include := snapshotNodeIncluded(node, interactiveOnly, selected)
	if include {
		line := formatSnapshotLine(node, depth)
		if line != "" {
			if node.referenceable() {
				ref := *nextRef
				*nextRef = *nextRef + 1
				line = formatSnapshotLineWithRef(ref, line)
				if strings.TrimSpace(node.Token) != "" {
					refMap[ref] = strings.TrimSpace(node.Token)
				}
			}
			*lines = append(*lines, line)
		}
	}
	for _, child := range node.Children {
		nextDepth := depth + 1
		if interactiveOnly && !include {
			nextDepth = depth
		}
		buildSnapshotTreeLines(child, interactiveOnly, selected, nextDepth, nextRef, refMap, lines)
	}
}

func buildCompactInteractiveSnapshotLines(nodes []*Node, nextRef *int, refMap map[int]string, lines *[]string) {
	for _, node := range nodes {
		if node == nil || !node.referenceable() {
			continue
		}
		line := formatSnapshotLine(node, 0)
		if line == "" {
			continue
		}
		ref := *nextRef
		*nextRef = *nextRef + 1
		line = formatSnapshotLineWithRef(ref, line)
		if strings.TrimSpace(node.Token) != "" {
			refMap[ref] = strings.TrimSpace(node.Token)
		}
		*lines = append(*lines, line)
	}
}

func snapshotNodeIncluded(node *Node, interactiveOnly bool, selected map[*Node]bool) bool {
	if node == nil {
		return false
	}
	if !interactiveOnly {
		return true
	}
	if !node.referenceable() {
		return false
	}
	if selected == nil {
		return true
	}
	return selected[node]
}

func (n *Node) referenceable() bool {
	if n == nil {
		return false
	}
	return n.Interactive || strings.TrimSpace(n.DefaultAction) != "" || strings.TrimSpace(n.Token) != ""
}

type interactiveSnapshotSelection struct {
	allowed map[*Node]bool
	ordered []*Node
}

func buildInteractiveSnapshotSelection(root *Node, interactiveOnly bool) *interactiveSnapshotSelection {
	if !interactiveOnly {
		return nil
	}
	nodes := collectReferenceableSnapshotNodes(root, nil)
	if len(nodes) <= interactiveSnapshotMaxRefs {
		return nil
	}
	type scoredNode struct {
		node  *Node
		score int
		order int
	}
	ranked := make([]scoredNode, 0, len(nodes))
	for idx, node := range nodes {
		ranked = append(ranked, scoredNode{
			node:  node,
			score: interactiveSnapshotPriority(node),
			order: idx,
		})
	}
	sort.SliceStable(ranked, func(i int, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].order < ranked[j].order
		}
		return ranked[i].score > ranked[j].score
	})
	selected := &interactiveSnapshotSelection{
		allowed: make(map[*Node]bool, interactiveSnapshotMaxRefs),
		ordered: make([]*Node, 0, interactiveSnapshotMaxRefs),
	}
	for _, item := range ranked[:interactiveSnapshotMaxRefs] {
		selected.allowed[item.node] = true
		selected.ordered = append(selected.ordered, item.node)
	}
	return selected
}

func collectReferenceableSnapshotNodes(node *Node, out []*Node) []*Node {
	if node == nil {
		return out
	}
	if node.referenceable() {
		out = append(out, node)
	}
	for _, child := range node.Children {
		out = collectReferenceableSnapshotNodes(child, out)
	}
	return out
}

func interactiveSnapshotPriority(node *Node) int {
	if node == nil {
		return 0
	}
	role := normalizeSnapshotMatchValue(node.Role)
	label := normalizeSnapshotMatchValue(snapshotNodeLabel(node))
	description := normalizeSnapshotMatchValue(node.Description)
	isWindowChrome := interactiveSnapshotIsWindowChrome(role, label, description)
	isShellNavigation := !isWindowChrome && interactiveSnapshotIsShellNavigation(role, label, description)

	score := 0
	switch {
	case snapshotContainsAny(role, "editable_text", "text_field", "text_area", "search_field", "combo_box", "document", "search", "editor"):
		score += 110
	case snapshotContainsAny(role, "button", "push_button", "switch", "check_box", "checkbox", "toggle"):
		score += 70
	case snapshotContainsAny(role, "tab", "radio", "link", "menu_item"):
		score += 45
	case snapshotContainsAny(role, "list_item", "tree_item", "row", "cell", "list", "menu", "outline"):
		score += 25
	default:
		score += 15
	}

	if label != "" {
		score += 10
	}
	if !isWindowChrome && !isShellNavigation && snapshotContainsAny(label, "message", "send", "reply", "search", "chat", "conversation", "contact", "compose", "new_chat", "new_message", "消息", "发送", "回复", "搜索", "聊天", "会话", "联系人", "新聊天", "新消息") {
		score += 50
	}
	if snapshotContainsAny(description, "focused", "selected", "active", "editable", "checked", "expanded") {
		score += 20
	}
	if snapshotContainsAny(description, "disabled", "unavailable", "offscreen", "invisible") {
		score -= 20
	}
	if isWindowChrome {
		score -= 90
	} else if isShellNavigation {
		score -= interactiveSnapshotShellNavigationPenalty(role, label)
	}
	return score
}

func interactiveSnapshotIsWindowChrome(role string, label string, description string) bool {
	if snapshotContainsAny(label, "close", "minimize", "maximize", "restore", "zoom", "full_screen", "fullscreen", "关闭", "最小化", "最大化", "还原", "缩放", "全屏") {
		return true
	}
	if snapshotContainsAny(description, "title_bar", "window_controls", "caption_bar", "titlebar", "窗口控制", "标题栏") {
		return true
	}
	return snapshotContainsAny(role, "title_bar", "caption", "window_controls")
}

func interactiveSnapshotIsShellNavigation(role string, label string, description string) bool {
	if snapshotContainsAny(label, "back", "forward", "reload", "refresh", "new_tab", "tab_search", "address_bar", "address_and_search_bar", "profile", "account", "settings", "preferences", "extensions", "extension", "developer_tools", "devtools", "sidebar", "后退", "前进", "刷新", "新标签页", "标签页搜索", "地址栏", "设置", "偏好设置", "扩展", "开发者工具", "侧边栏") {
		return true
	}
	if snapshotContainsAny(description, "toolbar", "navigation", "browser_chrome", "shell_toolbar", "工具栏", "导航栏", "浏览器壳层") {
		return true
	}
	return snapshotContainsAny(role, "toolbar", "navigation")
}

func interactiveSnapshotShellNavigationPenalty(role string, label string) int {
	penalty := 55
	if snapshotContainsAny(role, "editable_text", "text_field", "text_area", "search_field", "combo_box", "search") {
		penalty += 40
	}
	if snapshotContainsAny(label, "address_bar", "address_and_search_bar", "tab_search") {
		penalty += 10
	}
	return penalty
}

func normalizeSnapshotMatchValue(value string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(fields, "_")
}

func snapshotContainsAny(value string, parts ...string) bool {
	if value == "" {
		return false
	}
	for _, part := range parts {
		if strings.Contains(value, normalizeSnapshotMatchValue(part)) {
			return true
		}
	}
	return false
}

func formatSnapshotLine(node *Node, depth int) string {
	if node == nil {
		return ""
	}
	role := strings.TrimSpace(node.Role)
	if role == "" {
		role = "element"
	}
	label := snapshotNodeLabel(node)
	indent := strings.Repeat("  ", max(depth, 0))
	if label == "" {
		return fmt.Sprintf("%s[%s]", indent, role)
	}
	return fmt.Sprintf("%s[%s] %q", indent, role, label)
}

func formatSnapshotLineWithRef(ref int, line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indentLen := len(line) - len(trimmed)
	if indentLen <= 0 {
		return fmt.Sprintf("@%d %s", ref, line)
	}
	return fmt.Sprintf("%s@%d %s", line[:indentLen], ref, trimmed)
}

func snapshotNodeLabel(node *Node) string {
	if node == nil {
		return ""
	}
	for _, candidate := range []string{node.Name, node.Value, node.Description} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

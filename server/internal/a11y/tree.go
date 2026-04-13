package a11y

import (
	"fmt"
	"strings"
)

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
	buildSnapshotTreeLines(root, interactiveOnly, 0, &ref, refMap, &lines)
	if len(lines) == 0 {
		return "", nil
	}
	if len(refMap) == 0 {
		refMap = nil
	}
	return strings.Join(lines, "\n"), refMap
}

func buildSnapshotTreeLines(node *Node, interactiveOnly bool, depth int, nextRef *int, refMap map[int]string, lines *[]string) {
	if node == nil {
		return
	}
	include := !interactiveOnly || node.referenceable()
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
		buildSnapshotTreeLines(child, interactiveOnly, nextDepth, nextRef, refMap, lines)
	}
}

func (n *Node) referenceable() bool {
	if n == nil {
		return false
	}
	return n.Interactive || strings.TrimSpace(n.DefaultAction) != "" || strings.TrimSpace(n.Token) != ""
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

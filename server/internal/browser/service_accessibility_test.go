package browser

import (
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestDecodeAccessibilityTreeNodesSupportsNumericNodeIDs(t *testing.T) {
	raw := []byte(`{
		"nodes": [
			{
				"nodeId": 1,
				"ignored": false,
				"role": {"type":"role","value":"document"},
				"name": {"type":"computedString","value":"Example"},
				"childIds": [2]
			},
			{
				"nodeId": 2,
				"ignored": false,
				"role": {"type":"role","value":"link"},
				"name": {"type":"computedString","value":"Home"},
				"parentId": 1,
				"backendDOMNodeId": 42
			}
		]
	}`)

	nodes, err := decodeAccessibilityTreeNodes(raw)
	if err != nil {
		t.Fatalf("decodeAccessibilityTreeNodes() error = %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("decodeAccessibilityTreeNodes() = %d nodes, want 2", len(nodes))
	}
	if got := string(nodes[0].NodeID); got != "1" {
		t.Fatalf("root node id = %q, want 1", got)
	}
	if got := string(nodes[1].NodeID); got != "2" {
		t.Fatalf("child node id = %q, want 2", got)
	}
	if got := string(nodes[1].ParentID); got != "1" {
		t.Fatalf("child parent id = %q, want 1", got)
	}
	if len(nodes[0].ChildIDs) != 1 || string(nodes[0].ChildIDs[0]) != "2" {
		t.Fatalf("root child ids = %#v, want [\"2\"]", nodes[0].ChildIDs)
	}

	tree := newAXTreeBuilder(nodes).build()
	if !strings.Contains(tree, `[document] "Example"`) {
		t.Fatalf("tree = %q, want document node", tree)
	}
	if !strings.Contains(tree, `@1 [link] "Home"`) {
		t.Fatalf("tree = %q, want interactive link ref", tree)
	}
}

func TestSnapshotPageInfoFromInfoHandlesNil(t *testing.T) {
	snapshot := snapshotPageInfoFromInfo(nil)
	if snapshot.URL != "" || snapshot.Title != "" {
		t.Fatalf("snapshot = %#v, want empty fields", snapshot)
	}
}

func TestSnapshotPageInfoFromInfoCopiesFields(t *testing.T) {
	snapshot := snapshotPageInfoFromInfo(&proto.TargetTargetInfo{
		URL:   "https://example.com/docs",
		Title: "Example Docs",
	})
	if snapshot.URL != "https://example.com/docs" {
		t.Fatalf("snapshot.URL = %q, want example URL", snapshot.URL)
	}
	if snapshot.Title != "Example Docs" {
		t.Fatalf("snapshot.Title = %q, want title", snapshot.Title)
	}
}

func TestCheckedPageInfoRejectsNilPage(t *testing.T) {
	if _, err := checkedPageInfo(nil); err == nil {
		t.Fatal("checkedPageInfo(nil) error = nil, want error")
	}
}

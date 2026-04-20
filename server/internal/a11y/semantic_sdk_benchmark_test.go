package a11y

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkSemanticSDK_CompileLargeRawTree(b *testing.B) {
	sdk := newSemanticSDKWithHost(nil)
	raw := RawTree{
		WindowID: "win-bench",
		Title:    "Finder",
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: "Finder",
			Children: []*RawNode{
				{
					Role: "group",
					Name: "Toolbar",
					Children: []*RawNode{
						{Token: "token-delete", Role: "button", Name: "Delete", Interactive: true, Actions: []string{"AXPress"}},
						{Token: "token-rename", Role: "button", Name: "Rename", Interactive: true, Actions: []string{"AXPress"}},
					},
				},
				{
					Role:     "list",
					Name:     "Files",
					Children: benchmarkSemanticRows(600),
				},
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot, err := sdk.Compile(raw, CompileOptions{})
		if err != nil {
			b.Fatalf("Compile() error = %v", err)
		}
		if len(snapshot.Nodes) == 0 {
			b.Fatal("Compile() returned empty snapshot")
		}
	}
}

func BenchmarkSemanticSDK_ExpandSingleAdditionalPage(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		host := &fakeSemanticHost{
			captures: []RawTree{
				rawTreeWithRows("win-bench", "Finder", "token-row-1:file-1.txt", "token-row-2:file-2.txt"),
				rawTreeWithRows("win-bench", "Finder", "token-row-2:file-2.txt", "token-row-3:file-3.txt"),
			},
			captureIdx: 1,
		}
		sdk := newSemanticSDKWithHost(host)
		initial, err := sdk.Compile(host.captures[0], CompileOptions{})
		if err != nil {
			b.Fatalf("Compile() error = %v", err)
		}
		expanded, err := sdk.Expand(context.Background(), initial, Query{Name: "file-3.txt", Role: "row"})
		if err != nil {
			b.Fatalf("Expand() error = %v", err)
		}
		if len(expanded.Nodes) < 4 {
			b.Fatalf("len(expanded.Nodes) = %d, want >= 4", len(expanded.Nodes))
		}
	}
}

func benchmarkSemanticRows(count int) []*RawNode {
	rows := make([]*RawNode, 0, count)
	for idx := 0; idx < count; idx++ {
		rows = append(rows, &RawNode{
			Token:       fmt.Sprintf("token-row-%03d", idx),
			Role:        "row",
			Name:        fmt.Sprintf("file-%03d.txt", idx),
			Interactive: true,
			Actions:     []string{"AXPress"},
		})
	}
	return rows
}

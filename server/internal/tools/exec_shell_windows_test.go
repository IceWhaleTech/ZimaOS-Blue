//go:build windows

package tools

import "testing"

func TestBuildWindowsProcessTerminationOrder_PostOrder(t *testing.T) {
	childrenByParent := map[uint32][]uint32{
		42: {43, 44},
		43: {45},
		44: {46, 47},
	}

	got := buildWindowsProcessTerminationOrder(42, childrenByParent)
	want := []uint32{45, 43, 46, 47, 44, 42}

	if len(got) != len(want) {
		t.Fatalf("termination order length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("termination order[%d] = %d, want %d (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildWindowsProcessTerminationOrder_DeduplicatesCycles(t *testing.T) {
	childrenByParent := map[uint32][]uint32{
		100: {101, 102},
		101: {102},
		102: {100},
	}

	got := buildWindowsProcessTerminationOrder(100, childrenByParent)
	wantSet := map[uint32]struct{}{
		100: {},
		101: {},
		102: {},
	}

	if len(got) != len(wantSet) {
		t.Fatalf("termination order length = %d, want %d (%v)", len(got), len(wantSet), got)
	}
	seen := make(map[uint32]struct{}, len(got))
	for _, pid := range got {
		if _, ok := wantSet[pid]; !ok {
			t.Fatalf("unexpected pid %d in termination order %v", pid, got)
		}
		if _, dup := seen[pid]; dup {
			t.Fatalf("duplicate pid %d in termination order %v", pid, got)
		}
		seen[pid] = struct{}{}
	}
}

//go:build windows

package a11y

import (
	"context"
	"strings"
	"time"
)

func (b *windowsBackend) ResolveTarget(ctx context.Context, windowID string, selector TargetSelector) (TargetResolution, error) {
	hwnd, info, err := windowsResolveWindowFunc(windowID)
	if err != nil {
		return TargetResolution{}, err
	}
	start := time.Now()
	snapshot, cacheHit, _, err := b.ensureStructuredSnapshot(ctx, hwnd, info)
	if err != nil {
		return TargetResolution{}, err
	}
	result, err := ResolveSnapshotTarget(snapshot, selector)
	if err != nil {
		return TargetResolution{}, err
	}
	result.WindowID = info.ID
	result.CacheHit = cacheHit
	if result.QueryMS == 0 {
		result.QueryMS = time.Since(start).Milliseconds()
	}
	return result, nil
}

func (b *windowsBackend) UpdateSnapshotAfterAction(windowID string, token string, actType string, value string) {
	if b == nil || b.snapshots == nil {
		return
	}
	windowID = strings.TrimSpace(windowID)
	token = strings.TrimSpace(token)
	actType = strings.TrimSpace(strings.ToLower(actType))
	if windowID == "" {
		return
	}
	if actType == "type" && token != "" {
		if _, ok := b.snapshots.ApplyPatch(windowID, "value_changed", func(existing *Snapshot) *Snapshot {
			out := existing.Clone()
			for idx := range out.Nodes {
				if strings.TrimSpace(out.Nodes[idx].BackendToken) != token {
					continue
				}
				out.Nodes[idx].Value = strings.TrimSpace(value)
				if out.Nodes[idx].Description == "" {
					out.Nodes[idx].Description = "edited"
				}
				return out
			}
			return out
		}); ok {
			return
		}
	}
	_, _ = b.snapshots.MarkDirty(windowID, "action_completed")
}

func (b *windowsBackend) ensureStructuredSnapshot(_ context.Context, hwnd uintptr, info WindowInfo) (*Snapshot, bool, ActionTelemetry, error) {
	if b.snapshots != nil {
		if current, ok := b.snapshots.Current(info.ID); ok && current != nil && !current.Dirty {
			return current, true, ActionTelemetry{}, nil
		}
	}

	fetchStart := time.Now()
	var snapshot *Snapshot
	err := windowsWithCOM(func() error {
		root, err := windowsAccessibleObjectFromWindow(hwnd)
		if err != nil {
			return err
		}
		defer root.Release()

		visited := 0
		node := windowsBuildSnapshotNode(root, windowsCHILDIDSelf, hwnd, nil, 0, &visited)
		if node == nil {
			return NewError("backend_unavailable", "MSAA snapshot is empty", map[string]interface{}{"window_id": info.ID})
		}
		snapshot = BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
			WindowID: info.ID,
			Title:    info.Title,
			Mode:     "msaa",
		}, node)
		if snapshot == nil {
			return NewError("backend_unavailable", "MSAA snapshot is empty", map[string]interface{}{"window_id": info.ID})
		}
		return nil
	})
	if err != nil {
		return nil, false, ActionTelemetry{}, err
	}

	treeFetchMS := time.Since(fetchStart).Milliseconds()
	serializeStart := time.Now()
	if b.snapshots == nil {
		b.snapshots = NewSnapshotStore()
	}
	snapshot = b.snapshots.Swap(snapshot)
	telemetry := ActionTelemetry{
		SnapshotRevision: snapshot.Revision,
		NodeCount:        len(snapshot.Nodes),
		TreeFetchMS:      treeFetchMS,
		TreeSerializeMS:  time.Since(serializeStart).Milliseconds(),
		Fallbacks:        []string{"watch_unavailable"},
	}
	return snapshot, false, telemetry, nil
}

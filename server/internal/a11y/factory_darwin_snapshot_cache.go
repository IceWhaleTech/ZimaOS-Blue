//go:build darwin

package a11y

import (
	"context"
	"strings"
	"time"
)

func (b *darwinBackend) ResolveTarget(ctx context.Context, windowID string, selector TargetSelector) (TargetResolution, error) {
	record, err := darwinResolveWindowRecordForSnapshot(b, windowID)
	if err != nil {
		return TargetResolution{}, err
	}
	if err := b.ensureAccessibilityPermission(); err != nil {
		return TargetResolution{}, err
	}

	start := time.Now()
	snapshot, cacheHit, _, err := b.ensureStructuredSnapshot(ctx, record)
	if err != nil {
		return TargetResolution{}, err
	}
	result, err := ResolveSnapshotTarget(snapshot, selector)
	if err != nil {
		return TargetResolution{}, err
	}
	result.WindowID = record.ID
	if strings.TrimSpace(snapshot.WindowID) != "" {
		result.WindowID = snapshot.WindowID
	}
	result.CacheHit = cacheHit
	if result.QueryMS == 0 {
		result.QueryMS = time.Since(start).Milliseconds()
	}
	return result, nil
}

func (b *darwinBackend) CurrentStructuredSnapshot(windowID string) (*Snapshot, bool) {
	if b == nil || b.snapshots == nil {
		return nil, false
	}
	return b.snapshots.Current(strings.TrimSpace(windowID))
}

func (b *darwinBackend) UpdateSnapshotAfterAction(windowID string, token string, actType string, value string) {
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

func (b *darwinBackend) ensureStructuredSnapshot(_ context.Context, record darwinWindowRecord) (*Snapshot, bool, ActionTelemetry, error) {
	if b.snapshots != nil {
		if current, ok := b.snapshots.Current(record.ID); ok && current != nil && !current.Dirty {
			return current, true, ActionTelemetry{}, nil
		}
	}

	fetchStart := time.Now()
	if darwinAXUIElementCreateApplication == nil {
		return nil, false, ActionTelemetry{}, NewError("backend_unavailable", "AX application lookup is unavailable", map[string]interface{}{"window_id": record.ID})
	}
	app := darwinAXUIElementCreateApplication(int32(record.PID))
	if app == 0 {
		return nil, false, ActionTelemetry{}, NewError("backend_unavailable", "AX application lookup failed", map[string]interface{}{"window_id": record.ID})
	}
	defer darwinRelease(app)

	record, window := b.findSnapshotWindowElement(app, record)
	if window == 0 {
		return nil, false, ActionTelemetry{}, NewError("backend_unavailable", "AX window lookup failed", map[string]interface{}{"window_id": record.ID})
	}
	defer darwinRelease(window)

	b.prepareSnapshotElements(record.ID)
	defer b.finishSnapshotElementsBuild()
	defer func() {
		if r := recover(); r != nil {
			b.resetSnapshotElementsForWindow(record.ID)
			panic(r)
		}
	}()

	visited := 0
	root := b.buildSnapshotNode(window, 0, &visited)
	if root == nil {
		b.resetSnapshotElementsForWindow(record.ID)
		return nil, false, ActionTelemetry{}, NewError("backend_unavailable", "AX snapshot is empty", map[string]interface{}{"window_id": record.ID})
	}
	treeFetchMS := time.Since(fetchStart).Milliseconds()

	serializeStart := time.Now()
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: record.ID,
		Title:    record.Title,
		Mode:     "ax",
	}, root)
	if snapshot == nil {
		return nil, false, ActionTelemetry{}, NewError("backend_unavailable", "AX snapshot is empty", map[string]interface{}{"window_id": record.ID})
	}
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

func (b *darwinBackend) findSnapshotWindowElement(app uintptr, record darwinWindowRecord) (darwinWindowRecord, uintptr) {
	window := darwinFindWindowElementForSnapshot(app, record)
	if window != 0 {
		return record, window
	}
	refreshed, err := darwinRefreshWindowRecordForSnapshot(b, record)
	if err != nil {
		return record, 0
	}
	if strings.TrimSpace(refreshed.ID) == "" {
		return record, 0
	}
	window = darwinFindWindowElementForSnapshot(app, refreshed)
	if window != 0 {
		return refreshed, window
	}
	return record, 0
}

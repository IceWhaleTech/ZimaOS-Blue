//go:build windows

package a11y

import "context"

type windowsBackend struct {
	mediaDir  string
	snapshots *SnapshotStore
}

func DefaultHostBackend(mediaDir string) Backend {
	return &windowsBackend{
		mediaDir:  mediaDir,
		snapshots: NewSnapshotStore(),
	}
}

func (b *windowsBackend) HostOS() string { return "windows" }

func (b *windowsBackend) Capabilities(context.Context) (CapabilitiesResult, error) {
	return CapabilitiesResult{
		HostOS:           b.HostOS(),
		SupportedActions: append([]string(nil), SupportedActions...),
		Permissions: []PermissionStatus{
			{Name: "msaa", Granted: true, Required: true},
		},
	}, nil
}

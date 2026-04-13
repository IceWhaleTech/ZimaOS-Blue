package a11y

import "context"

func windowsSnapshotWithImageFallback(
	ctx context.Context,
	hostOS string,
	windowID string,
	interactiveOnly bool,
	resolve func(string) (uintptr, WindowInfo, error),
	snapshot func(string, uintptr, WindowInfo, bool) (SnapshotResult, error),
	capture func(context.Context, string) (ScreenshotResult, error),
) (SnapshotResult, error) {
	if resolve == nil {
		return SnapshotResult{HostOS: hostOS}, NewError("backend_unavailable", "window resolver is unavailable", nil)
	}
	if snapshot == nil {
		return SnapshotResult{HostOS: hostOS}, NewError("backend_unavailable", "MSAA snapshot runtime is unavailable", nil)
	}
	hwnd, info, err := resolve(windowID)
	if err != nil {
		return SnapshotResult{HostOS: hostOS}, err
	}
	result, err := snapshot(hostOS, hwnd, info, interactiveOnly)
	if err != nil {
		return SnapshotResult{HostOS: hostOS}, enrichSnapshotErrorWithImage(ctx, err, info.ID, capture)
	}
	return attachSnapshotImage(ctx, result, capture), nil
}

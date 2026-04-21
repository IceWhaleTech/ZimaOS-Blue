//go:build darwin

package a11y

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

type darwinElementRef struct {
	Element uintptr
}

type darwinPoint struct {
	X float64
	Y float64
}

type darwinSize struct {
	Width  float64
	Height float64
}

type darwinRect struct {
	Origin darwinPoint
	Size   darwinSize
}

type darwinWindowRecord struct {
	ID      string
	Title   string
	AppName string
	PID     int
	Layer   int
	Focused bool
	Bounds  darwinRect
}

var (
	darwinRuntimeOnce sync.Once

	darwinCoreGraphicsHandle                     uintptr
	darwinCoreFoundationHandle                   uintptr
	darwinImageIOHandle                          uintptr
	darwinApplicationServices                    uintptr
	darwinCGWindowListCopyInfo                   func(option uint32, relativeToWindow uint32) uintptr
	darwinCGWindowListCreateImage                func(screenBounds darwinRect, listOption uint32, windowID uint32, imageOption uint32) uintptr
	darwinCGRectMakeWithDictionaryRepresentation func(dict uintptr, rect *darwinRect) bool
	darwinCFArrayGetCount                        func(array uintptr) int64
	darwinCFArrayGetValueAtIndex                 func(array uintptr, index int64) uintptr
	darwinCFDictionaryGetValue                   func(dict uintptr, key uintptr) uintptr
	darwinCFDataCreateMutable                    func(allocator uintptr, capacity int64) uintptr
	darwinCFDataGetLength                        func(data uintptr) int64
	darwinCFDataGetBytePtr                       func(data uintptr) uintptr
	darwinCFGetTypeID                            func(ref uintptr) uintptr
	darwinCFStringGetTypeID                      func() uintptr
	darwinCFNumberGetTypeID                      func() uintptr
	darwinCFBooleanGetTypeID                     func() uintptr
	darwinCFBooleanGetValue                      func(boolean uintptr) bool
	darwinCFNumberGetValue                       func(number uintptr, numberType int32, valuePtr unsafe.Pointer) bool
	darwinCFStringCreate                         func(allocator uintptr, bytes *byte, encoding uint32) uintptr
	darwinCFStringGetLength                      func(str uintptr) int64
	darwinCFStringGetCString                     func(str uintptr, buffer *byte, bufferSize int64, encoding uint32) bool
	darwinCFRetain                               func(ref uintptr) uintptr
	darwinCFRelease                              func(ref uintptr)
	darwinAXUIElementCreateApplication           func(pid int32) uintptr
	darwinAXUIElementCreateSystemWide            func() uintptr
	darwinAXUIElementCopyAttributeValue          func(element uintptr, attribute uintptr, out *uintptr) int32
	darwinAXUIElementCopyActionNames             func(element uintptr, out *uintptr) int32
	darwinAXUIElementPerformAction               func(element uintptr, action uintptr) int32
	darwinAXUIElementSetAttributeValue           func(element uintptr, attribute uintptr, value uintptr) int32
	darwinAXUIElementIsAttributeSettable         func(element uintptr, attribute uintptr, settable *bool) int32
	darwinAXUIElementGetPid                      func(element uintptr, pid *int32) int32
	darwinAXValueGetTypeID                       func() uintptr
	darwinAXValueGetType                         func(value uintptr) int32
	darwinAXValueGetValue                        func(value uintptr, valueType int32, out unsafe.Pointer) bool
	darwinCGEventCreateMouseEvent                func(source uintptr, mouseType uint32, point darwinPoint, button uint32) uintptr
	darwinCGEventCreateScrollWheelEvent          func(source uintptr, units uint32, wheelCount uint32, wheel1 int32, wheel2 int32) uintptr
	darwinCGEventSetIntegerValueField            func(event uintptr, field uint32, value int64)
	darwinCGEventCreateKeyboardEvent             func(source uintptr, virtualKey uint16, keyDown bool) uintptr
	darwinCGEventSetFlags                        func(event uintptr, flags uint64)
	darwinCGEventKeyboardSetUnicodeString        func(event uintptr, length uint64, chars *uint16)
	darwinCGEventPost                            func(tap uint32, event uintptr)
	darwinCGWarpMouseCursorPosition              func(point darwinPoint) int32
	darwinCGImageDestinationCreateWithData       func(data uintptr, typeRef uintptr, count uintptr, options uintptr) uintptr
	darwinCGImageDestinationAddImage             func(dest uintptr, image uintptr, properties uintptr)
	darwinCGImageDestinationFinalize             func(dest uintptr) bool
	darwinResolveWindowRecordForCapture          = func(b *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return b.resolveWindowRecord(strings.TrimSpace(windowID))
	}
	darwinRefreshWindowRecordForCapture = func(b *darwinBackend, current darwinWindowRecord) (darwinWindowRecord, error) {
		return b.refreshWindowRecord(current)
	}
	darwinCaptureWindowImage    = darwinRunWindowCapture
	darwinCaptureWindowPNGBytes = darwinCaptureWindowRecordPNGBytes
	darwinScreenshotRetrySleep  = darwinSleepWithContext
)

const (
	darwinCGWindowListOptionAll             = 0
	darwinUTF8Encoding                      = 0x08000100
	darwinCGWindowListOptionOnScreenOnly    = 1
	darwinCGWindowListOptionIncludingWindow = 1 << 3
	darwinCGWindowListExcludeDesktop        = 16
	darwinCGWindowImageBoundsIgnoreFraming  = 1 << 0
	darwinCGWindowImageBestResolution       = 1 << 3
	darwinCFNumberSInt64Type                = 4
	darwinScreenshotMaxAttempts             = 3
	darwinScreenshotRetryDelay              = 180 * time.Millisecond
)

func initDarwinRuntime() {
	darwinRuntimeOnce.Do(func() {
		var err error
		darwinCoreGraphicsHandle, err = purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		darwinCoreFoundationHandle, err = purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		darwinImageIOHandle, err = purego.Dlopen("/System/Library/Frameworks/ImageIO.framework/ImageIO", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		darwinApplicationServices, err = purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&darwinCGWindowListCopyInfo, darwinCoreGraphicsHandle, "CGWindowListCopyWindowInfo")
		purego.RegisterLibFunc(&darwinCGWindowListCreateImage, darwinCoreGraphicsHandle, "CGWindowListCreateImage")
		purego.RegisterLibFunc(&darwinCGRectMakeWithDictionaryRepresentation, darwinCoreGraphicsHandle, "CGRectMakeWithDictionaryRepresentation")
		purego.RegisterLibFunc(&darwinCFArrayGetCount, darwinCoreFoundationHandle, "CFArrayGetCount")
		purego.RegisterLibFunc(&darwinCFArrayGetValueAtIndex, darwinCoreFoundationHandle, "CFArrayGetValueAtIndex")
		purego.RegisterLibFunc(&darwinCFDictionaryGetValue, darwinCoreFoundationHandle, "CFDictionaryGetValue")
		purego.RegisterLibFunc(&darwinCFDataCreateMutable, darwinCoreFoundationHandle, "CFDataCreateMutable")
		purego.RegisterLibFunc(&darwinCFDataGetLength, darwinCoreFoundationHandle, "CFDataGetLength")
		purego.RegisterLibFunc(&darwinCFDataGetBytePtr, darwinCoreFoundationHandle, "CFDataGetBytePtr")
		purego.RegisterLibFunc(&darwinCFGetTypeID, darwinCoreFoundationHandle, "CFGetTypeID")
		purego.RegisterLibFunc(&darwinCFStringGetTypeID, darwinCoreFoundationHandle, "CFStringGetTypeID")
		purego.RegisterLibFunc(&darwinCFNumberGetTypeID, darwinCoreFoundationHandle, "CFNumberGetTypeID")
		purego.RegisterLibFunc(&darwinCFBooleanGetTypeID, darwinCoreFoundationHandle, "CFBooleanGetTypeID")
		purego.RegisterLibFunc(&darwinCFBooleanGetValue, darwinCoreFoundationHandle, "CFBooleanGetValue")
		purego.RegisterLibFunc(&darwinCFNumberGetValue, darwinCoreFoundationHandle, "CFNumberGetValue")
		purego.RegisterLibFunc(&darwinCFStringCreate, darwinCoreFoundationHandle, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&darwinCFStringGetLength, darwinCoreFoundationHandle, "CFStringGetLength")
		purego.RegisterLibFunc(&darwinCFStringGetCString, darwinCoreFoundationHandle, "CFStringGetCString")
		purego.RegisterLibFunc(&darwinCFRetain, darwinCoreFoundationHandle, "CFRetain")
		purego.RegisterLibFunc(&darwinCFRelease, darwinCoreFoundationHandle, "CFRelease")
		purego.RegisterLibFunc(&darwinAXUIElementCreateApplication, darwinApplicationServices, "AXUIElementCreateApplication")
		purego.RegisterLibFunc(&darwinAXUIElementCreateSystemWide, darwinApplicationServices, "AXUIElementCreateSystemWide")
		purego.RegisterLibFunc(&darwinAXUIElementCopyAttributeValue, darwinApplicationServices, "AXUIElementCopyAttributeValue")
		purego.RegisterLibFunc(&darwinAXUIElementCopyActionNames, darwinApplicationServices, "AXUIElementCopyActionNames")
		purego.RegisterLibFunc(&darwinAXUIElementPerformAction, darwinApplicationServices, "AXUIElementPerformAction")
		purego.RegisterLibFunc(&darwinAXUIElementSetAttributeValue, darwinApplicationServices, "AXUIElementSetAttributeValue")
		purego.RegisterLibFunc(&darwinAXUIElementIsAttributeSettable, darwinApplicationServices, "AXUIElementIsAttributeSettable")
		purego.RegisterLibFunc(&darwinAXUIElementGetPid, darwinApplicationServices, "AXUIElementGetPid")
		purego.RegisterLibFunc(&darwinAXValueGetTypeID, darwinApplicationServices, "AXValueGetTypeID")
		purego.RegisterLibFunc(&darwinAXValueGetType, darwinApplicationServices, "AXValueGetType")
		purego.RegisterLibFunc(&darwinAXValueGetValue, darwinApplicationServices, "AXValueGetValue")
		purego.RegisterLibFunc(&darwinCGEventCreateMouseEvent, darwinCoreGraphicsHandle, "CGEventCreateMouseEvent")
		purego.RegisterLibFunc(&darwinCGEventCreateScrollWheelEvent, darwinCoreGraphicsHandle, "CGEventCreateScrollWheelEvent")
		purego.RegisterLibFunc(&darwinCGEventSetIntegerValueField, darwinCoreGraphicsHandle, "CGEventSetIntegerValueField")
		purego.RegisterLibFunc(&darwinCGEventCreateKeyboardEvent, darwinCoreGraphicsHandle, "CGEventCreateKeyboardEvent")
		purego.RegisterLibFunc(&darwinCGEventSetFlags, darwinCoreGraphicsHandle, "CGEventSetFlags")
		purego.RegisterLibFunc(&darwinCGEventKeyboardSetUnicodeString, darwinCoreGraphicsHandle, "CGEventKeyboardSetUnicodeString")
		purego.RegisterLibFunc(&darwinCGEventPost, darwinCoreGraphicsHandle, "CGEventPost")
		purego.RegisterLibFunc(&darwinCGWarpMouseCursorPosition, darwinCoreGraphicsHandle, "CGWarpMouseCursorPosition")
		purego.RegisterLibFunc(&darwinCGImageDestinationCreateWithData, darwinImageIOHandle, "CGImageDestinationCreateWithData")
		purego.RegisterLibFunc(&darwinCGImageDestinationAddImage, darwinImageIOHandle, "CGImageDestinationAddImage")
		purego.RegisterLibFunc(&darwinCGImageDestinationFinalize, darwinImageIOHandle, "CGImageDestinationFinalize")
	})
}

func (b *darwinBackend) listWindows(context.Context) ([]WindowInfo, error) {
	records, err := b.listWindowRecords()
	if err != nil {
		return nil, err
	}
	return darwinWindowInfosFromRecords(records), nil
}

func (b *darwinBackend) listAllWindows(context.Context) ([]WindowInfo, error) {
	records, err := b.listAllWindowRecords()
	if err != nil {
		return nil, err
	}
	return darwinWindowInfosFromRecords(records), nil
}

func darwinWindowInfosFromRecords(records []darwinWindowRecord) []WindowInfo {
	windows := make([]WindowInfo, 0, len(records))
	for _, record := range records {
		windows = append(windows, WindowInfo{
			ID:      record.ID,
			Title:   record.Title,
			AppName: record.AppName,
			PID:     record.PID,
			Focused: record.Focused,
			Layer:   record.Layer,
			Bounds: Rect{
				X:      record.Bounds.Origin.X,
				Y:      record.Bounds.Origin.Y,
				Width:  record.Bounds.Size.Width,
				Height: record.Bounds.Size.Height,
			},
		})
	}
	return windows
}

func (b *darwinBackend) screenshot(ctx context.Context, windowID string) (ScreenshotResult, error) {
	record, err := darwinResolveWindowRecordForCapture(b, strings.TrimSpace(windowID))
	if err != nil {
		return ScreenshotResult{HostOS: b.HostOS()}, err
	}
	return b.screenshotWindowRecord(ctx, record)
}

func (b *darwinBackend) screenshotWindowRecord(ctx context.Context, record darwinWindowRecord) (ScreenshotResult, error) {
	dir := filepath.Join(os.TempDir(), "zimaos-blue", "computer-use")
	if strings.TrimSpace(b.mediaDir) != "" {
		dir = filepath.Join(b.mediaDir, "computer-use")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return ScreenshotResult{HostOS: b.HostOS()}, fmt.Errorf("create screenshot dir: %w", err)
	}
	current := record
	var lastErr error
	for attempt := 0; attempt < darwinScreenshotMaxAttempts; attempt++ {
		path := filepath.Join(dir, fmt.Sprintf("host-window-%s.png", current.ID))
		output, captureErr := darwinCaptureWindowImage(ctx, current.ID, path)
		if captureErr == nil {
			return ScreenshotResult{
				HostOS:    b.HostOS(),
				WindowID:  current.ID,
				ImagePath: path,
				Message:   "Host screenshot captured",
			}, nil
		}
		lastErr = fmt.Errorf("screencapture failed: %s: %w", strings.TrimSpace(output), captureErr)
		if attempt == darwinScreenshotMaxAttempts-1 {
			break
		}
		if sleepErr := darwinScreenshotRetrySleep(ctx, darwinScreenshotRetryDelay); sleepErr != nil {
			return ScreenshotResult{HostOS: b.HostOS()}, sleepErr
		}
		if refreshed, refreshErr := darwinRefreshWindowRecordForCapture(b, current); refreshErr == nil && strings.TrimSpace(refreshed.ID) != "" {
			current = refreshed
		}
	}
	return ScreenshotResult{HostOS: b.HostOS()}, lastErr
}

func (b *darwinBackend) screenshotForGrounding(ctx context.Context, windowID string) (ScreenshotResult, error) {
	record, err := darwinResolveWindowRecordForCapture(b, strings.TrimSpace(windowID))
	if err != nil {
		return ScreenshotResult{HostOS: b.HostOS()}, err
	}
	imagePNG, err := darwinCaptureWindowPNGBytes(ctx, record)
	if err != nil {
		return ScreenshotResult{HostOS: b.HostOS()}, err
	}
	return ScreenshotResult{
		HostOS:     b.HostOS(),
		WindowID:   record.ID,
		ImageBytes: append([]byte(nil), imagePNG...),
		Message:    "Host screenshot captured",
	}, nil
}

func darwinWindowInfoFromDictionary(dict uintptr) darwinWindowRecord {
	record := darwinWindowRecord{}
	record.ID = strconv.Itoa(int(darwinCFNumberValue(darwinDictionaryValue(dict, "kCGWindowNumber"))))
	record.PID = int(darwinCFNumberValue(darwinDictionaryValue(dict, "kCGWindowOwnerPID")))
	record.Layer = int(darwinCFNumberValue(darwinDictionaryValue(dict, "kCGWindowLayer")))
	record.AppName = darwinCFStringValue(darwinDictionaryValue(dict, "kCGWindowOwnerName"))
	record.Title = darwinCFStringValue(darwinDictionaryValue(dict, "kCGWindowName"))
	record.Bounds = darwinCFDictionaryRectValue(darwinDictionaryValue(dict, "kCGWindowBounds"))
	if strings.TrimSpace(record.Title) == "" {
		record.Title = record.AppName
	}
	return record
}

func darwinDictionaryValue(dict uintptr, key string) uintptr {
	if dict == 0 || darwinCFDictionaryGetValue == nil {
		return 0
	}
	keyRef := darwinCFStringRef(key)
	if keyRef == 0 {
		return 0
	}
	defer darwinRelease(keyRef)
	return darwinCFDictionaryGetValue(dict, keyRef)
}

func darwinCFStringRef(value string) uintptr {
	if darwinCFStringCreate == nil {
		return 0
	}
	bytes := append([]byte(value), 0)
	return darwinCFStringCreate(0, &bytes[0], darwinUTF8Encoding)
}

func darwinCFStringValue(ref uintptr) string {
	if ref == 0 || darwinCFStringGetLength == nil || darwinCFStringGetCString == nil {
		return ""
	}
	length := darwinCFStringGetLength(ref)
	if length <= 0 {
		return ""
	}
	buf := make([]byte, length*4+1)
	if !darwinCFStringGetCString(ref, &buf[0], int64(len(buf)), darwinUTF8Encoding) {
		return ""
	}
	if idx := bytes.IndexByte(buf, 0); idx >= 0 {
		buf = buf[:idx]
	}
	return strings.TrimSpace(string(buf))
}

func darwinCFNumberValue(ref uintptr) int64 {
	if ref == 0 || darwinCFNumberGetValue == nil {
		return 0
	}
	var value int64
	if !darwinCFNumberGetValue(ref, darwinCFNumberSInt64Type, unsafe.Pointer(&value)) {
		return 0
	}
	return value
}

func darwinCFDictionaryRectValue(ref uintptr) darwinRect {
	var rect darwinRect
	if ref == 0 || darwinCGRectMakeWithDictionaryRepresentation == nil {
		return rect
	}
	if !darwinCGRectMakeWithDictionaryRepresentation(ref, &rect) {
		return darwinRect{}
	}
	return rect
}

func darwinRunWindowCapture(ctx context.Context, windowID string, path string) (string, error) {
	return darwinCLIFallback.captureWindow(ctx, windowID, path)
}

func darwinCaptureWindowRecordPNGBytes(_ context.Context, record darwinWindowRecord) ([]byte, error) {
	initDarwinRuntime()
	if darwinCGWindowListCreateImage == nil || darwinCFDataCreateMutable == nil || darwinCFDataGetLength == nil || darwinCFDataGetBytePtr == nil || darwinCGImageDestinationCreateWithData == nil || darwinCGImageDestinationAddImage == nil || darwinCGImageDestinationFinalize == nil {
		return nil, fmt.Errorf("native screenshot bytes capture is unavailable")
	}
	windowID, err := strconv.ParseUint(strings.TrimSpace(record.ID), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("parse window id for screenshot: %w", err)
	}
	if record.Bounds.Size.Width <= 0 || record.Bounds.Size.Height <= 0 {
		return nil, fmt.Errorf("target window has no visible bounds")
	}
	imageRef := darwinCGWindowListCreateImage(
		record.Bounds,
		darwinCGWindowListOptionIncludingWindow,
		uint32(windowID),
		darwinCGWindowImageBoundsIgnoreFraming|darwinCGWindowImageBestResolution,
	)
	if imageRef == 0 {
		return nil, fmt.Errorf("native screenshot returned empty image")
	}
	defer darwinRelease(imageRef)
	dataRef := darwinCFDataCreateMutable(0, 0)
	if dataRef == 0 {
		return nil, fmt.Errorf("create mutable PNG data failed")
	}
	defer darwinRelease(dataRef)
	pngType := darwinCFStringRef("public.png")
	if pngType == 0 {
		return nil, fmt.Errorf("create png type ref failed")
	}
	defer darwinRelease(pngType)
	destRef := darwinCGImageDestinationCreateWithData(dataRef, pngType, 1, 0)
	if destRef == 0 {
		return nil, fmt.Errorf("create image destination failed")
	}
	defer darwinRelease(destRef)
	darwinCGImageDestinationAddImage(destRef, imageRef, 0)
	if !darwinCGImageDestinationFinalize(destRef) {
		return nil, fmt.Errorf("finalize image destination failed")
	}
	length := darwinCFDataGetLength(dataRef)
	if length <= 0 {
		return nil, fmt.Errorf("native screenshot produced empty png data")
	}
	dataPtr := darwinCFDataGetBytePtr(dataRef)
	if dataPtr == 0 {
		return nil, fmt.Errorf("native screenshot png bytes unavailable")
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(dataPtr)), int(length))...), nil
}

func darwinSleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

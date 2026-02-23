//go:build darwin

// send_sharing_darwin.go implements a fallback iMessage sender using
// the imessage:// URL scheme + CGEvent keyboard simulation.
// This is used when the primary AppleScript sender fails due to
// Automation permission denial (-1743).
//
// Flow:
//  1. Record frontmost app (to restore focus later)
//  2. Open imessage:// URL (pre-fills recipient + body, no permission needed)
//  3. Wait for Messages.app to become active
//  4. Press Return via CGEvent to send the message
//  5. Hide Messages.app and restore previous frontmost app
//
// Requires: Accessibility permission (System Settings > Privacy > Accessibility)
// for CGEvent keyboard simulation.
package imessage

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"go.uber.org/zap"
)

// --- CoreGraphics CGEvent (keyboard simulation) ---

var (
	cgOnce                     sync.Once
	cgLoaded                   bool
	cgEventCreateKeyboardEvent func(source uintptr, virtualKey uint16, keyDown bool) uintptr
	cgEventPost                func(tap uint32, event uintptr)
	cfRelease                  func(cf uintptr)
)

func initCoreGraphics() {
	cgOnce.Do(func() {
		cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_LAZY)
		if err != nil {
			return
		}
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&cgEventCreateKeyboardEvent, cg, "CGEventCreateKeyboardEvent")
		purego.RegisterLibFunc(&cgEventPost, cg, "CGEventPost")
		purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
		cgLoaded = true
	})
}

// --- ObjC selectors for NSWorkspace / NSRunningApplication ---

var (
	nsOnce sync.Once

	selAlloc                        objc.SEL
	selSharedWorkspace              objc.SEL
	selOpenURL                      objc.SEL
	selURLWithString                objc.SEL
	selFrontmostApplication         objc.SEL
	selRunningApplicationsWithBID   objc.SEL
	selActivateWithOptions          objc.SEL
	selHide                         objc.SEL
	selBundleIdentifier             objc.SEL
	selIsActive                     objc.SEL
	selCount                        objc.SEL
	selObjectAtIndex                objc.SEL
	selStringWithUTF8String         objc.SEL
	selUTF8String                   objc.SEL
)

const messagesBundleID = "com.apple.MobileSMS"

func initObjCSelectors() {
	nsOnce.Do(func() {
		// Load AppKit framework
		_, _ = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL)

		selAlloc = objc.RegisterName("alloc")
		selSharedWorkspace = objc.RegisterName("sharedWorkspace")
		selOpenURL = objc.RegisterName("openURL:")
		selURLWithString = objc.RegisterName("URLWithString:")
		selFrontmostApplication = objc.RegisterName("frontmostApplication")
		selRunningApplicationsWithBID = objc.RegisterName("runningApplicationsWithBundleIdentifier:")
		selActivateWithOptions = objc.RegisterName("activateWithOptions:")
		selHide = objc.RegisterName("hide")
		selBundleIdentifier = objc.RegisterName("bundleIdentifier")
		selIsActive = objc.RegisterName("isActive")
		selCount = objc.RegisterName("count")
		selObjectAtIndex = objc.RegisterName("objectAtIndex:")
		selStringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
		selUTF8String = objc.RegisterName("UTF8String")
	})
}

// nsStr creates an NSString from a Go string.
func nsStr(s string) objc.ID {
	cstr := append([]byte(s), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	return cls.Send(selStringWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
}

// goStr reads a Go string from an NSString.
func goStr(ns objc.ID) string {
	if ns == 0 {
		return ""
	}
	ptr := objc.Send[uintptr](ns, selUTF8String)
	if ptr == 0 {
		return ""
	}
	return cStr(ptr)
}

// cStr reads a null-terminated C string from a raw pointer.
//
//go:nocheckptr
func cStr(ptr uintptr) string {
	length := 0
	for {
		b := *(*byte)(unsafe.Add(unsafe.Pointer(ptr), length))
		if b == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length))
}

// --- Main fallback sender ---

// sendViaSharingService sends an iMessage using the imessage:// URL scheme
// + CGEvent Return key press. No osascript involved.
func (c *Channel) sendViaSharingService(ctx context.Context, recipient, content string) error {
	initCoreGraphics()
	initObjCSelectors()

	if !cgLoaded {
		return fmt.Errorf("CoreGraphics not available on this system")
	}

	c.logger.Info("sending iMessage via URL scheme fallback",
		zap.String("recipient", recipient),
		zap.Int("content_length", len(content)))

	// Step 1: Record the current frontmost app so we can restore focus
	prevApp := getFrontmostApp()
	c.logger.Debug("recorded frontmost app", zap.String("bundle_id", prevApp))

	// Step 2: Open imessage:// URL — pre-fills recipient + body
	u := fmt.Sprintf("imessage://send?service=iMessage&recipient=%s&body=%s",
		url.QueryEscape(recipient), url.QueryEscape(content))

	nsURL := objc.ID(objc.GetClass("NSURL")).Send(selURLWithString, nsStr(u))
	if nsURL == 0 {
		return fmt.Errorf("failed to create NSURL for imessage:// scheme")
	}

	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(selSharedWorkspace)
	ok := objc.Send[bool](workspace, selOpenURL, nsURL)
	if !ok {
		// Fallback to `open` command
		c.logger.Debug("NSWorkspace.openURL failed, trying open command")
		cmd := exec.CommandContext(ctx, "open", u)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to open imessage:// URL: %s: %w", string(out), err)
		}
	}

	// Step 3: Wait for Messages.app to become frontmost (up to 5s)
	c.logger.Debug("waiting for Messages.app to activate...")
	if err := waitForMessagesActive(ctx, 5*time.Second); err != nil {
		hideMessages()
		restoreFrontmostApp(prevApp)
		return fmt.Errorf("Messages.app did not activate: %w", err)
	}

	// Small delay for the compose field to be ready
	time.Sleep(500 * time.Millisecond)

	// Step 4: Press Return via CGEvent to send the message
	c.logger.Debug("pressing Return via CGEvent...")
	if err := pressReturnKey(); err != nil {
		hideMessages()
		restoreFrontmostApp(prevApp)
		return fmt.Errorf("CGEvent Return key failed: %w", err)
	}

	// Brief pause to let the message send
	time.Sleep(500 * time.Millisecond)

	// Step 5: Hide Messages.app and restore previous app
	hideMessages()
	restoreFrontmostApp(prevApp)

	c.logger.Info("iMessage sent via URL scheme + CGEvent fallback",
		zap.String("recipient", recipient))
	return nil
}

// pressReturnKey sends a Return key press via CGEvent.
// Requires Accessibility permission for the calling process.
func pressReturnKey() error {
	const kVKReturn uint16 = 0x24
	const kCGHIDEventTap uint32 = 0

	keyDown := cgEventCreateKeyboardEvent(0, kVKReturn, true)
	if keyDown == 0 {
		return fmt.Errorf("CGEventCreateKeyboardEvent failed — grant Accessibility permission to \"%s\" in System Settings > Privacy & Security > Accessibility", tccAppName())
	}
	keyUp := cgEventCreateKeyboardEvent(0, kVKReturn, false)

	cgEventPost(kCGHIDEventTap, keyDown)
	cgEventPost(kCGHIDEventTap, keyUp)
	cfRelease(keyDown)
	if keyUp != 0 {
		cfRelease(keyUp)
	}
	return nil
}

// getFrontmostApp returns the bundle identifier of the current frontmost app.
func getFrontmostApp() string {
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(selSharedWorkspace)
	frontApp := workspace.Send(selFrontmostApplication)
	if frontApp == 0 {
		return ""
	}
	bid := frontApp.Send(selBundleIdentifier)
	return goStr(bid)
}

// waitForMessagesActive polls until Messages.app is the frontmost app.
func waitForMessagesActive(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		bid := getFrontmostApp()
		if bid == messagesBundleID {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for Messages.app to become active")
}

// hideMessages hides the Messages.app window using NSRunningApplication.hide.
func hideMessages() {
	bidStr := nsStr(messagesBundleID)
	cls := objc.ID(objc.GetClass("NSRunningApplication"))
	apps := cls.Send(selRunningApplicationsWithBID, bidStr)
	if apps == 0 {
		return
	}
	count := objc.Send[int](apps, selCount)
	if count == 0 {
		return
	}
	app := apps.Send(selObjectAtIndex, uintptr(0))
	if app != 0 {
		app.Send(selHide)
	}
}

// restoreFrontmostApp activates the previously frontmost app.
func restoreFrontmostApp(bundleID string) {
	if bundleID == "" || bundleID == messagesBundleID {
		return
	}
	bidStr := nsStr(bundleID)
	cls := objc.ID(objc.GetClass("NSRunningApplication"))
	apps := cls.Send(selRunningApplicationsWithBID, bidStr)
	if apps == 0 {
		return
	}
	count := objc.Send[int](apps, selCount)
	if count == 0 {
		return
	}
	app := apps.Send(selObjectAtIndex, uintptr(0))
	if app != 0 {
		// NSApplicationActivateIgnoringOtherApps = 1 << 1 = 2
		objc.Send[bool](app, selActivateWithOptions, uintptr(2))
	}
}

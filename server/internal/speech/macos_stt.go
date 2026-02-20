//go:build darwin

package speech

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const ProviderMacOSNative stt.ProviderType = "macos-native"

// ObjC selectors (registered once)
var (
	sttOnce sync.Once

	selAlloc                       objc.SEL
	selInit                        objc.SEL
	selInitWithLocale              objc.SEL
	selInitWithURL                 objc.SEL
	selAuthorizationStatus         objc.SEL
	selRequestAuthorization        objc.SEL
	selIsAvailable                 objc.SEL
	selSupportsOnDeviceRecognition objc.SEL
	selRecognitionTaskWithRequest  objc.SEL
	selSetShouldReportPartial      objc.SEL
	selSetRequiresOnDevice         objc.SEL
	selBestTranscription           objc.SEL
	selFormattedString             objc.SEL
	selIsFinal                     objc.SEL
	selInitWithLocaleIdentifier    objc.SEL
	selFileURLWithPath             objc.SEL
	selLocalizedDescription        objc.SEL
	selStringWithUTF8String        objc.SEL
	selUTF8String                  objc.SEL
)

func initSTTSelectors() {
	sttOnce.Do(func() {
		// Load Speech framework so ObjC runtime knows about SFSpeechRecognizer
		_, err := purego.Dlopen("/System/Library/Frameworks/Speech.framework/Speech", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			slog.Error("[macos-stt] failed to load Speech.framework", "error", err)
			return
		}

		selAlloc = objc.RegisterName("alloc")
		selInit = objc.RegisterName("init")
		selInitWithLocale = objc.RegisterName("initWithLocale:")
		selInitWithURL = objc.RegisterName("initWithURL:")
		selAuthorizationStatus = objc.RegisterName("authorizationStatus")
		selRequestAuthorization = objc.RegisterName("requestAuthorization:")
		selIsAvailable = objc.RegisterName("isAvailable")
		selSupportsOnDeviceRecognition = objc.RegisterName("supportsOnDeviceRecognition")
		selRecognitionTaskWithRequest = objc.RegisterName("recognitionTaskWithRequest:resultHandler:")
		selSetShouldReportPartial = objc.RegisterName("setShouldReportPartialResults:")
		selSetRequiresOnDevice = objc.RegisterName("setRequiresOnDeviceRecognition:")
		selBestTranscription = objc.RegisterName("bestTranscription")
		selFormattedString = objc.RegisterName("formattedString")
		selIsFinal = objc.RegisterName("isFinal")
		selInitWithLocaleIdentifier = objc.RegisterName("initWithLocaleIdentifier:")
		selFileURLWithPath = objc.RegisterName("fileURLWithPath:")
		selLocalizedDescription = objc.RegisterName("localizedDescription")
		selStringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
		selUTF8String = objc.RegisterName("UTF8String")
	})
}

// nsString creates an NSString from a Go string via UTF8.
func nsString(s string) objc.ID {
	cstr := append([]byte(s), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	return cls.Send(selStringWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
}

// goString reads a Go string from an NSString.
//
//go:nosplit
func goString(nsStr objc.ID) string {
	if nsStr == 0 {
		return ""
	}
	sel := objc.RegisterName("UTF8String")
	ptr := objc.Send[uintptr](nsStr, sel)
	if ptr == 0 {
		return ""
	}
	return cstring(ptr)
}

// cstring reads a null-terminated C string from a raw pointer.
//
//go:nocheckptr
func cstring(ptr uintptr) string {
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

// Package-level authorization state, set by RequestSTTAuthorization() in main().
var (
	sttAuthMu     sync.Mutex
	sttAuthStatus int = -1 // -1 = not yet requested, 0..3 = SFSpeechRecognizerAuthorizationStatus
	sttAuthErr    error
)

// mainDoneCh signals RunMainRunLoop to stop.
var mainDoneCh = make(chan struct{})

// GCD dispatch support — used by SubmitToMainThread to dispatch closures
// to the main thread via dispatch_async_f. Works in both CLI mode
// (RunMainRunLoop pumps NSRunLoop which drains GCD main queue) and
// Tauri mode (Tauri's Cocoa event loop drains GCD main queue).
var (
	gcdOnce          sync.Once
	dispatchMainQueue uintptr // dispatch_queue_t from dispatch_get_main_queue()
	dispatchAsyncF   func(queue uintptr, context uintptr, work uintptr)
)

// pendingWork stores Go closures keyed by an incrementing ID.
// dispatch_async_f passes the ID as context, the C callback looks it up.
var (
	pendingWorkMu sync.Mutex
	pendingWorkID uintptr
	pendingWork   = make(map[uintptr]func())
)

func initGCD() {
	gcdOnce.Do(func() {
		libdispatch, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY)
		if err != nil {
			slog.Error("[macos-stt] failed to open libSystem for GCD", "err", err)
			return
		}
		// dispatch_get_main_queue() is an inline that returns &_dispatch_main_q
		dispatchMainQueue, err = purego.Dlsym(libdispatch, "_dispatch_main_q")
		if err != nil {
			slog.Error("[macos-stt] failed to find _dispatch_main_q", "err", err)
			return
		}
		purego.RegisterLibFunc(&dispatchAsyncF, libdispatch, "dispatch_async_f")
	})
}

// gcdCallback is the C function pointer passed to dispatch_async_f.
// It receives the pendingWork ID as context, looks up and runs the closure.
var gcdCallbackPtr = purego.NewCallback(func(ctx uintptr) {
	pendingWorkMu.Lock()
	fn, ok := pendingWork[ctx]
	if ok {
		delete(pendingWork, ctx)
	}
	pendingWorkMu.Unlock()
	if ok {
		fn()
	}
})

// SubmitToMainThread dispatches a closure to execute on the main thread
// via GCD dispatch_async_f. Non-blocking — returns immediately after queuing.
// Works in both CLI mode (RunMainRunLoop) and Tauri mode (Cocoa event loop).
func SubmitToMainThread(fn func()) {
	initGCD()
	if dispatchAsyncF == nil {
		// Fallback: run inline if GCD init failed (shouldn't happen on macOS)
		slog.Warn("[macos-stt] GCD not available, running inline")
		fn()
		return
	}
	pendingWorkMu.Lock()
	pendingWorkID++
	id := pendingWorkID
	pendingWork[id] = fn
	pendingWorkMu.Unlock()

	dispatchAsyncF(dispatchMainQueue, id, gcdCallbackPtr)
}

// StopMainRunLoop signals RunMainRunLoop to return.
func StopMainRunLoop() {
	select {
	case mainDoneCh <- struct{}{}:
	default:
	}
}

// RunMainRunLoop pumps the Cocoa main run loop on thread 0 forever.
// Must be called from main() after RequestSTTAuthorization().
// The server should be started on a separate goroutine before calling this.
// Returns when StopMainRunLoop() is called.
// In Tauri mode this is NOT called — Tauri owns the Cocoa event loop,
// and SubmitToMainThread uses GCD dispatch_async which Tauri drains.
func RunMainRunLoop() {
	selCurrentRunLoop := objc.RegisterName("currentRunLoop")
	selRunUntilDate := objc.RegisterName("runUntilDate:")
	selDateWithInterval := objc.RegisterName("dateWithTimeIntervalSinceNow:")
	nsRunLoopCls := objc.ID(objc.GetClass("NSRunLoop"))
	nsDateCls := objc.ID(objc.GetClass("NSDate"))
	runLoop := nsRunLoopCls.Send(selCurrentRunLoop)

	slog.Info("[macos-stt] main run loop started on thread 0")
	for {
		// Check for stop signal
		select {
		case <-mainDoneCh:
			slog.Info("[macos-stt] main run loop stopped")
			return
		default:
		}
		// Pump the run loop for 50ms to let GCD/AppKit/Speech callbacks fire.
		// GCD dispatch_async work items are delivered through this run loop.
		futureDate := nsDateCls.Send(selDateWithInterval, 0.05)
		runLoop.Send(selRunUntilDate, futureDate)
	}
}

// RequestSTTAuthorization runs NSApp initialization and speech recognition
// authorization on thread 0. Must be called from main() before the server
// starts. The result is stored in package-level state that Initialize() reads.
//
// Returns the authorization status (0=NotDetermined, 1=Denied, 2=Restricted, 3=Authorized).
func RequestSTTAuthorization() (int, error) {
	sttAuthMu.Lock()
	defer sttAuthMu.Unlock()

	initSTTSelectors()

	cls := objc.GetClass("SFSpeechRecognizer")
	if cls == 0 {
		sttAuthErr = fmt.Errorf("SFSpeechRecognizer class not found")
		return 0, sttAuthErr
	}

	// Check current status first
	status := objc.Send[int](objc.ID(cls), selAuthorizationStatus)
	slog.Info("[macos-stt] authorization status", "status", status)

	if status == 0 {
		// Preflight checks
		if !hasSpeechUsageDescription() {
			slog.Warn("[macos-stt] NSSpeechRecognitionUsageDescription not found")
			sttAuthErr = fmt.Errorf("macOS native speech recognition unavailable: " +
				"NSSpeechRecognitionUsageDescription missing from Info.plist")
			return 0, sttAuthErr
		}
		if !verifyCodeSignature() {
			slog.Warn("[macos-stt] no valid code signature")
			sttAuthErr = fmt.Errorf("macOS native speech recognition unavailable: " +
				"no valid code signature. Sign the binary with codesign")
			return 0, sttAuthErr
		}

		slog.Info("[macos-stt] requesting speech recognition authorization via NSApp...")
		newStatus, err := requestAuthorizationViaNSApp(objc.ID(cls))
		if err != nil {
			sttAuthErr = err
			return 0, err
		}
		status = newStatus
	}

	sttAuthStatus = status
	if status == 1 || status == 2 {
		sttAuthErr = fmt.Errorf("speech recognition denied or restricted (status=%d); "+
			"grant permission in System Settings > Privacy & Security > Speech Recognition", status)
	}
	return status, sttAuthErr
}

type MacOSNativeSTT struct {
	initialized       bool
	requireOnDevice   bool // user preference: force on-device only
	onDeviceSupported bool // cached at init time
	mu                sync.Mutex
}

func NewMacOSNativeSTT() *MacOSNativeSTT {
	return &MacOSNativeSTT{}
}

// Initialize checks the authorization status that was set by
// RequestSTTAuthorization() in main(). It does NOT request authorization
// itself — that must happen on thread 0 before the server starts.
func (p *MacOSNativeSTT) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		return nil
	}

	sttAuthMu.Lock()
	status := sttAuthStatus
	authErr := sttAuthErr
	sttAuthMu.Unlock()

	if status == -1 {
		// RequestSTTAuthorization() was never called (e.g. non-macOS build path)
		return fmt.Errorf("macOS STT authorization not initialized; call RequestSTTAuthorization() from main() first")
	}
	if authErr != nil {
		return authErr
	}
	if status != 3 {
		return fmt.Errorf("speech recognition not authorized (status=%d)", status)
	}

	initSTTSelectors()
	p.initialized = true

	// Probe on-device support (SFSpeechRecognizer is thread-safe for this query)
	cls := objc.ID(objc.GetClass("SFSpeechRecognizer"))
	if cls != 0 {
		recognizer := cls.Send(selAlloc).Send(selInit)
		if recognizer != 0 {
			p.onDeviceSupported = objc.Send[bool](recognizer, selSupportsOnDeviceRecognition)
		}
	}

	return nil
}

// RequestSTTAuthorizationEmbedded is like RequestSTTAuthorization but for
// embedded mode (Tauri). It skips NSApp initialization (Tauri already owns
// the Cocoa event loop) and just checks/requests authorization status.
// The requestAuthorization: callback fires via Tauri's run loop.
// Can be called from any thread — does not require thread 0.
func RequestSTTAuthorizationEmbedded() (int, error) {
	sttAuthMu.Lock()
	defer sttAuthMu.Unlock()

	initSTTSelectors()

	cls := objc.GetClass("SFSpeechRecognizer")
	if cls == 0 {
		sttAuthErr = fmt.Errorf("SFSpeechRecognizer class not found")
		return 0, sttAuthErr
	}

	// Check current status
	status := objc.Send[int](objc.ID(cls), selAuthorizationStatus)
	slog.Info("[macos-stt] embedded authorization status", "status", status)

	if status == 0 {
		// Preflight checks
		if !hasSpeechUsageDescription() {
			slog.Warn("[macos-stt] NSSpeechRecognitionUsageDescription not found")
			sttAuthErr = fmt.Errorf("macOS native speech recognition unavailable: " +
				"NSSpeechRecognitionUsageDescription missing from Info.plist")
			return 0, sttAuthErr
		}
		if !verifyCodeSignature() {
			slog.Warn("[macos-stt] no valid code signature")
			sttAuthErr = fmt.Errorf("macOS native speech recognition unavailable: " +
				"no valid code signature. Sign the binary with codesign")
			return 0, sttAuthErr
		}

		slog.Info("[macos-stt] requesting speech recognition authorization (embedded)...")

		authCh := make(chan int, 1)
		block := objc.NewBlock(func(_ objc.Block, s int) {
			slog.Info("[macos-stt] embedded authorization callback", "status", s)
			select {
			case authCh <- s:
			default:
			}
		})
		objc.ID(cls).Send(selRequestAuthorization, block)

		// Wait for callback — Tauri's run loop delivers it
		select {
		case status = <-authCh:
			block.Release()
		case <-time.After(30 * time.Second):
			block.Release()
			sttAuthErr = fmt.Errorf("speech recognition authorization timed out")
			return 0, sttAuthErr
		}
	}

	sttAuthStatus = status
	if status == 1 || status == 2 {
		sttAuthErr = fmt.Errorf("speech recognition denied or restricted (status=%d); "+
			"grant permission in System Settings > Privacy & Security > Speech Recognition", status)
	}
	return status, sttAuthErr
}

// requestAuthorizationViaNSApp runs [NSApp run] on the current thread
// (must be thread 0) to let TCC present the consent dialog.
//
// REQUIRES: runtime.LockOSThread() in init() pins main goroutine to thread 0.
// Call chain: main() → cobra → runServer() → Initialize() is synchronous.
func requestAuthorizationViaNSApp(sfClass objc.ID) (int, error) {
	// Load AppKit
	_, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("failed to load AppKit: %w", err)
	}

	appCls := objc.GetClass("NSApplication")
	if appCls == 0 {
		return 0, fmt.Errorf("NSApplication class not found")
	}

	selSharedApp := objc.RegisterName("sharedApplication")
	selSetActivationPolicy := objc.RegisterName("setActivationPolicy:")
	selFinishLaunching := objc.RegisterName("finishLaunching")

	nsApp := objc.ID(appCls).Send(selSharedApp)
	if nsApp == 0 {
		return 0, fmt.Errorf("[NSApplication sharedApplication] returned nil")
	}

	// Accessory policy — no dock icon, no menu bar.
	// Regular policy causes AppKit to initialize menus which triggers
	// WritingToolsUI dlopen and crashes in Go's ObjC runtime environment.
	// Accessory is sufficient for TCC — the consent dialog is system-level.
	nsApp.Send(selSetActivationPolicy, uintptr(1))

	// finishLaunching posts NSApplicationDidFinishLaunchingNotification
	// and completes NSApp initialization without entering a full run loop.
	// This avoids the menu bar initialization that crashes with Regular policy.
	nsApp.Send(selFinishLaunching)

	slog.Info("[macos-stt] NSApp initialized (accessory), requesting authorization...")

	authCh := make(chan int, 1)

	// Request authorization — callback fires on a background GCD queue
	block := objc.NewBlock(func(_ objc.Block, status int) {
		slog.Info("[macos-stt] authorization callback", "status", status)
		select {
		case authCh <- status:
		default:
		}
	})
	sfClass.Send(selRequestAuthorization, block)

	// Pump the NSRunLoop on thread 0 so GCD callbacks and TCC can work.
	// This is lighter than [NSApp run] and avoids menu initialization.
	selCurrentRunLoop := objc.RegisterName("currentRunLoop")
	selRunUntilDate := objc.RegisterName("runUntilDate:")
	selDateWithInterval := objc.RegisterName("dateWithTimeIntervalSinceNow:")
	nsRunLoopCls := objc.ID(objc.GetClass("NSRunLoop"))
	nsDateCls := objc.ID(objc.GetClass("NSDate"))
	runLoop := nsRunLoopCls.Send(selCurrentRunLoop)

	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case status := <-authCh:
			block.Release()
			return status, nil
		default:
		}
		if time.Now().After(deadline) {
			block.Release()
			return 0, fmt.Errorf("speech recognition authorization timed out")
		}
		// Pump run loop for 100ms
		futureDate := nsDateCls.Send(selDateWithInterval, 0.1)
		runLoop.Send(selRunUntilDate, futureDate)
	}
}

func (p *MacOSNativeSTT) Name() string             { return "macOS Native" }
func (p *MacOSNativeSTT) Type() stt.ProviderType    { return ProviderMacOSNative }
func (p *MacOSNativeSTT) Available() bool            { return true }
func (p *MacOSNativeSTT) MaxDuration() time.Duration { return 60 * time.Second }

// SetRequireOnDevice sets whether to force on-device recognition only.
func (p *MacOSNativeSTT) SetRequireOnDevice(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requireOnDevice = v
}

// RequireOnDevice returns the current on-device preference.
func (p *MacOSNativeSTT) RequireOnDevice() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requireOnDevice
}

// SupportsOnDevice returns the cached on-device recognition support status.
// The value is probed once during Initialize().
func (p *MacOSNativeSTT) SupportsOnDevice() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.onDeviceSupported
}

// DictationAvailable checks if macOS Dictation is enabled in System Settings.
// On macOS Ventura+ (13+), reads com.apple.assistant.support → "Dictation Enabled".
// Falls back to com.apple.HIToolbox → AppleDictationAutoEnable for older macOS.
// NSUserDefaults is thread-safe — no need to dispatch to mainWorkCh.
func (p *MacOSNativeSTT) DictationAvailable() bool {
	initSTTSelectors()
	cls := objc.ID(objc.GetClass("NSUserDefaults"))
	if cls == 0 {
		return false
	}
	selStandardUserDefaults := objc.RegisterName("standardUserDefaults")
	selSynchronize := objc.RegisterName("synchronize")
	selPersistentDomain := objc.RegisterName("persistentDomainForName:")
	selObjectForKey := objc.RegisterName("objectForKey:")
	selBoolValue := objc.RegisterName("boolValue")

	defaults := cls.Send(selStandardUserDefaults)
	if defaults == 0 {
		return false
	}
	defaults.Send(selSynchronize)

	// Primary: com.apple.assistant.support → "Dictation Enabled" (macOS Ventura+)
	domain := defaults.Send(selPersistentDomain, nsString("com.apple.assistant.support"))
	if domain != 0 {
		val := domain.Send(selObjectForKey, nsString("Dictation Enabled"))
		if val != 0 {
			return objc.Send[bool](val, selBoolValue)
		}
	}

	// Fallback: com.apple.HIToolbox → AppleDictationAutoEnable (older macOS)
	domain = defaults.Send(selPersistentDomain, nsString("com.apple.HIToolbox"))
	if domain == 0 {
		return false
	}
	val := domain.Send(selObjectForKey, nsString("AppleDictationAutoEnable"))
	if val == 0 {
		return false
	}
	return objc.Send[bool](val, selBoolValue)
}

func (p *MacOSNativeSTT) SupportedFormats() []stt.AudioFormat {
	return []stt.AudioFormat{stt.FormatWAV, stt.FormatMP3, stt.FormatFLAC, stt.FormatOGG}
}

// OfflineDictationLanguages returns the list of locale codes that have offline
// dictation installed (Installed=1 in "Offline Dictation Status").
// NSUserDefaults is thread-safe — no need to dispatch to mainWorkCh.
func (p *MacOSNativeSTT) OfflineDictationLanguages() []string {
	initSTTSelectors()
	cls := objc.ID(objc.GetClass("NSUserDefaults"))
	if cls == 0 {
		return nil
	}
	defaults := cls.Send(objc.RegisterName("standardUserDefaults"))
	if defaults == 0 {
		return nil
	}
	defaults.Send(objc.RegisterName("synchronize"))

	domain := defaults.Send(objc.RegisterName("persistentDomainForName:"), nsString("com.apple.assistant.support"))
	if domain == 0 {
		return nil
	}
	selObjectForKey := objc.RegisterName("objectForKey:")
	offlineDict := domain.Send(selObjectForKey, nsString("Offline Dictation Status"))
	if offlineDict == 0 {
		return nil
	}

	// Get all keys from the NSDictionary
	selAllKeys := objc.RegisterName("allKeys")
	selCount := objc.RegisterName("count")
	selObjAtIndex := objc.RegisterName("objectAtIndex:")
	selBoolValue := objc.RegisterName("boolValue")

	keys := offlineDict.Send(selAllKeys) // NSArray
	if keys == 0 {
		return nil
	}
	count := int(objc.Send[uintptr](keys, selCount))
	var installed []string
	for i := 0; i < count; i++ {
		key := keys.Send(selObjAtIndex, uintptr(i))
		if key == 0 {
			continue
		}
		// Get the sub-dictionary for this locale
		langDict := offlineDict.Send(selObjectForKey, key)
		if langDict == 0 {
			continue
		}
		installedVal := langDict.Send(selObjectForKey, nsString("Installed"))
		if installedVal == 0 {
			continue
		}
		if objc.Send[bool](installedVal, selBoolValue) {
			locale := goString(key)
			if locale != "" {
				installed = append(installed, locale)
			}
		}
	}
	return installed
}

func (p *MacOSNativeSTT) Transcribe(ctx context.Context, req *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	if err := p.Initialize(); err != nil {
		return nil, err
	}

	ext := ".wav"
	switch req.Format {
	case stt.FormatMP3:
		ext = ".mp3"
	case stt.FormatOGG:
		ext = ".ogg"
	case stt.FormatFLAC:
		ext = ".flac"
	case stt.FormatWebM:
		ext = ".webm"
	}

	tmpFile, err := os.CreateTemp("", "macos-stt-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, req.Audio); err != nil {
		return nil, fmt.Errorf("failed to write audio: %w", err)
	}
	tmpFile.Close()

	// Apple Speech framework is picky about audio formats. Use afconvert
	// to re-encode into 16kHz 16-bit mono WAV which it always accepts.
	// This also handles browser-produced WAV that may have quirks.
	audioPath := tmpFile.Name()
	if needsConversion(ext) {
		wavPath := tmpFile.Name() + ".converted.wav"
		if err := afconvertToWAV(ctx, audioPath, wavPath); err != nil {
			slog.Warn("[macos-stt] afconvert failed, using original file", "error", err)
		} else {
			defer os.Remove(wavPath)
			audioPath = wavPath
		}
	}

	locale := langToLocale(req.Language)
	text, err := p.recognize(ctx, audioPath, locale)
	if err != nil {
		return nil, err
	}

	return &stt.TranscribeResponse{
		Text:     text,
		Language: req.Language,
	}, nil
}

func (p *MacOSNativeSTT) TranscribeStream(ctx context.Context, req *stt.TranscribeRequest, callback stt.StreamCallback) error {
	resp, err := p.Transcribe(ctx, req)
	if err != nil {
		return err
	}
	return callback(resp)
}

func (p *MacOSNativeSTT) recognize(_ context.Context, audioPath, locale string) (string, error) {
	// Speech framework calls are dispatched to the main thread via GCD
	// (dispatch_async_f). In CLI mode, RunMainRunLoop pumps NSRunLoop which
	// drains the GCD main queue. In Tauri mode, Tauri's Cocoa event loop
	// drains it. We wait for the result on this goroutine via a channel.

	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)

	SubmitToMainThread(func() {
		initSTTSelectors()

		// Create SFSpeechRecognizer
		cls := objc.ID(objc.GetClass("SFSpeechRecognizer"))
		var recognizer objc.ID
		if locale != "" {
			nsCls := objc.ID(objc.GetClass("NSLocale"))
			nsLocale := nsCls.Send(selAlloc).Send(selInitWithLocaleIdentifier, nsString(locale))
			recognizer = cls.Send(selAlloc).Send(selInitWithLocale, nsLocale)
		} else {
			recognizer = cls.Send(selAlloc).Send(selInit)
		}
		if recognizer == 0 {
			ch <- result{err: fmt.Errorf("failed to create SFSpeechRecognizer")}
			return
		}

		avail := objc.Send[bool](recognizer, selIsAvailable)
		if !avail {
			ch <- result{err: fmt.Errorf("speech recognizer not available for locale %q", locale)}
			return
		}

		// Create NSURL from file path
		urlCls := objc.ID(objc.GetClass("NSURL"))
		audioURL := urlCls.Send(selFileURLWithPath, nsString(audioPath))
		if audioURL == 0 {
			ch <- result{err: fmt.Errorf("failed to create NSURL for %s", audioPath)}
			return
		}

		// Create SFSpeechURLRecognitionRequest
		reqCls := objc.ID(objc.GetClass("SFSpeechURLRecognitionRequest"))
		request := reqCls.Send(selAlloc).Send(selInitWithURL, audioURL)
		if request == 0 {
			ch <- result{err: fmt.Errorf("failed to create recognition request")}
			return
		}
		request.Send(selSetShouldReportPartial, false)

		// Check on-device support and apply user preference
		onDevice := objc.Send[bool](recognizer, selSupportsOnDeviceRecognition)
		requireOnDevice := p.RequireOnDevice()
		if requireOnDevice && onDevice {
			request.Send(selSetRequiresOnDevice, true)
		}
		slog.Info("[macos-stt] starting recognition task",
			"audioPath", audioPath, "locale", locale,
			"onDeviceSupported", onDevice, "requireOnDevice", requireOnDevice)

		// Block callback: ^(SFSpeechRecognitionResult *res, NSError *err)
		var lastText string
		block := objc.NewBlock(func(_ objc.Block, res objc.ID, nsErr objc.ID) {
			slog.Info("[macos-stt] block callback invoked", "res", res, "err", nsErr)
			if res != 0 {
				transcription := res.Send(selBestTranscription)
				if transcription != 0 {
					text := goString(transcription.Send(selFormattedString))
					if text != "" {
						lastText = text
					}
				}
				isFinal := objc.Send[bool](res, selIsFinal)
				if isFinal {
					select {
					case ch <- result{text: lastText}:
					default:
					}
					return
				}
			}
			if nsErr != 0 {
				desc := goString(nsErr.Send(selLocalizedDescription))
				slog.Warn("[macos-stt] recognition error", "desc", desc, "lastText", lastText)
				if lastText != "" {
					select {
					case ch <- result{text: lastText}:
					default:
					}
				} else {
					var err error = fmt.Errorf("recognition error: %s", desc)
					// Detect on-device unavailable errors
					if requireOnDevice && isOnDeviceError(desc) {
						err = &OnDeviceUnavailableError{Locale: locale, Detail: desc}
					} else if se := friendlySpeechError(desc); se != nil {
						err = se
					}
					select {
					case ch <- result{err: err}:
					default:
					}
				}
				return
			}
		})

		// Start recognition task — callbacks fire on GCD queues,
		// but the main run loop must be active for Speech framework internals.
		recognizer.Send(selRecognitionTaskWithRequest, request, block)

		// Set a timeout to release the block and send an error if no result comes
		go func() {
			time.Sleep(60 * time.Second)
			block.Release()
			select {
			case ch <- result{err: fmt.Errorf("speech recognition timed out")}:
			default:
			}
		}()
	})

	// Wait for result from the callback (runs on this goroutine, not thread 0)
	r := <-ch
	if r.err != nil {
		return "", r.err
	}
	return strings.TrimSpace(r.text), nil
}

func (p *MacOSNativeSTT) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialized = false
}

// OnDeviceUnavailableError indicates on-device recognition failed for the locale.
type OnDeviceUnavailableError struct {
	Locale string
	Detail string
}

func (e *OnDeviceUnavailableError) Error() string {
	return fmt.Sprintf("on-device recognition not available for locale %q: %s", e.Locale, e.Detail)
}

// isOnDeviceError checks if an NSError description indicates on-device model unavailability.
func isOnDeviceError(desc string) bool {
	d := strings.ToLower(desc)
	return strings.Contains(d, "on-device") ||
		strings.Contains(d, "offline") ||
		strings.Contains(d, "not supported for this locale") ||
		strings.Contains(d, "siri and dictation") ||
		strings.Contains(d, "no speech detected") // on-device may silently fail with this
}

// friendlySpeechError maps cryptic Apple Speech framework errors to SpeechError with error codes.
// Returns nil if the error is not recognized.
func friendlySpeechError(desc string) *SpeechError {
	d := strings.ToLower(desc)
	switch {
	case strings.Contains(d, "cannot open"):
		return &SpeechError{Code: "audio_invalid", Message: desc}
	case strings.Contains(d, "no speech detected"):
		return &SpeechError{Code: "no_speech", Message: desc}
	case strings.Contains(d, "not available"):
		return &SpeechError{Code: "service_unavailable", Message: desc}
	case strings.Contains(d, "rate limit"):
		return &SpeechError{Code: "rate_limit", Message: desc}
	default:
		return nil
	}
}

// needsConversion returns true for formats that Apple Speech framework cannot handle directly.
// WAV is not included because the frontend now converts to WAV before sending.
func needsConversion(ext string) bool {
	switch ext {
	case ".webm", ".ogg":
		return true
	default:
		return false
	}
}

// afconvertToWAV uses macOS built-in afconvert to re-encode audio to 16kHz 16-bit mono WAV.
func afconvertToWAV(ctx context.Context, src, dst string) error {
	cmd := exec.CommandContext(ctx, "afconvert",
		"-f", "WAVE", "-d", "LEI16@16000", "-c", "1", src, dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("afconvert: %w (output: %s)", err, string(output))
	}
	return nil
}

// langToLocale maps short language codes to BCP-47 locale identifiers.
func langToLocale(lang string) string {
	switch lang {
	case "en":
		return "en-US"
	case "zh", "cmn":
		return "zh-CN"
	case "ja":
		return "ja-JP"
	case "ko":
		return "ko-KR"
	case "ru":
		return "ru-RU"
	case "ar":
		return "ar-SA"
	case "th":
		return "th-TH"
	case "hi":
		return "hi-IN"
	case "fr":
		return "fr-FR"
	case "de":
		return "de-DE"
	case "es":
		return "es-ES"
	case "pt":
		return "pt-BR"
	case "it":
		return "it-IT"
	default:
		return ""
	}
}


// hasSpeechUsageDescription checks if NSBundle.mainBundle has the
// NSSpeechRecognitionUsageDescription key in its Info.plist.
// TCC reads this from the bundle (or embedded __TEXT,__info_plist) and will
// abort() the process if it's missing when requestAuthorization: is called.
// This preflight lets us fail gracefully instead of crashing.
func hasSpeechUsageDescription() bool {
	cls := objc.GetClass("NSBundle")
	if cls == 0 {
		return false
	}
	selMainBundle := objc.RegisterName("mainBundle")
	selObjectForKey := objc.RegisterName("objectForInfoDictionaryKey:")
	bundle := objc.ID(cls).Send(selMainBundle)
	if bundle == 0 {
		return false
	}
	key := nsString("NSSpeechRecognitionUsageDescription")
	val := bundle.Send(selObjectForKey, key)
	slog.Info("[macos-stt] preflight NSSpeechRecognitionUsageDescription", "found", val != 0)
	return val != 0
}

// verifyCodeSignature uses Security.framework SecStaticCodeCheckValidity to check
// if the current binary has a valid code signature.
func verifyCodeSignature() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}

	sec, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_LAZY)
	if err != nil {
		return false
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY)
	if err != nil {
		return false
	}

	var cfURLCreateWithFileSystemPath func(alloc uintptr, path uintptr, style int32, isDir bool) uintptr
	var secStaticCodeCreateWithPath func(url uintptr, flags uint32, codeOut *uintptr) int32
	var secStaticCodeCheckValidity func(code uintptr, flags uint32, requirement uintptr) int32
	var cfRelease func(cf uintptr)

	purego.RegisterLibFunc(&cfURLCreateWithFileSystemPath, cf, "CFURLCreateWithFileSystemPath")
	purego.RegisterLibFunc(&secStaticCodeCreateWithPath, sec, "SecStaticCodeCreateWithPath")
	purego.RegisterLibFunc(&secStaticCodeCheckValidity, sec, "SecStaticCodeCheckValidity")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")

	pathNS := nsString(exe)
	// kCFStringEncodingPOSIXPath = 0
	url := cfURLCreateWithFileSystemPath(0, uintptr(pathNS), 0, false)
	if url == 0 {
		return false
	}
	defer cfRelease(url)

	var staticCode uintptr
	if rc := secStaticCodeCreateWithPath(url, 0, &staticCode); rc != 0 {
		return false
	}
	defer cfRelease(staticCode)

	// Validate with flags=0 (kSecCSDefaultFlags). SecStaticCodeCheckValidity
	// only checks the Mach-O code signature and ignores data appended after
	// the Mach-O boundary (e.g., pack-dist trailer), so this works even with
	// the self-extracting dist format.
	rc := secStaticCodeCheckValidity(staticCode, 0, 0)
	slog.Info("[macos-stt] code signature check", "path", exe, "result", rc)
	return rc == 0
}

// GetTCCAppName returns the app name that macOS TCC associates the speech
// recognition permission with. TCC binds to the parent app's bundle ID:
//   - Tauri app → "Blue"
//   - CLI (launcher forces Terminal.app) → "Terminal"
func GetTCCAppName() string {
	// Check if running inside a .app bundle (Tauri)
	exe, err := os.Executable()
	if err == nil && strings.Contains(exe, ".app/Contents/MacOS/") {
		return "Blue"
	}
	// CLI mode — launcher forces Terminal.app via osascript
	return "Terminal"
}

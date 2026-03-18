//go:build darwin

package speech

import (
	"context"
	"encoding/binary"
	"errors"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	selCancel                      objc.SEL
	selRetain                      objc.SEL
	selRelease                     objc.SEL

	// AVFoundation selectors for buffer-based recognition
	selInitWithFormat       objc.SEL // AVAudioPCMBuffer initWithPCMFormat:frameCapacity:
	selInitStdFormatSR      objc.SEL // AVAudioFormat initStandardFormatWithSampleRate:channels:
	selAppendAudioPCMBuffer objc.SEL // SFSpeechAudioBufferRecognitionRequest appendAudioPCMBuffer:
	selEndAudio             objc.SEL // SFSpeechAudioBufferRecognitionRequest endAudio
	selFloatChannelData     objc.SEL // AVAudioPCMBuffer floatChannelData
	selFrameLength          objc.SEL // AVAudioPCMBuffer frameLength
	selSetFrameLength       objc.SEL // AVAudioPCMBuffer setFrameLength:
)

var (
	hasSpeechUsageDescriptionFunc     = hasSpeechUsageDescription
	hasMicrophoneUsageDescriptionFunc = hasMicrophoneUsageDescription
	verifyCodeSignatureFunc           = verifyCodeSignature
)

func initSTTSelectors() {
	sttOnce.Do(func() {
		// Load Speech framework so ObjC runtime knows about SFSpeechRecognizer
		_, err := purego.Dlopen("/System/Library/Frameworks/Speech.framework/Speech", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			slog.Error("[macos-stt] failed to load Speech.framework", "error", err)
			return
		}
		// Load AVFoundation for AVAudioPCMBuffer / AVAudioFormat
		_, _ = purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)

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
		selCancel = objc.RegisterName("cancel")
		selRetain = objc.RegisterName("retain")
		selRelease = objc.RegisterName("release")

		// AVFoundation selectors for buffer-based recognition
		selInitWithFormat = objc.RegisterName("initWithPCMFormat:frameCapacity:")
		selInitStdFormatSR = objc.RegisterName("initStandardFormatWithSampleRate:channels:")
		selAppendAudioPCMBuffer = objc.RegisterName("appendAudioPCMBuffer:")
		selEndAudio = objc.RegisterName("endAudio")
		selFloatChannelData = objc.RegisterName("floatChannelData")
		selFrameLength = objc.RegisterName("frameLength")
		selSetFrameLength = objc.RegisterName("setFrameLength:")
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
	ptr := objc.Send[uintptr](nsStr, selUTF8String)
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

func retainObject(id objc.ID) objc.ID {
	if id == 0 {
		return 0
	}
	return id.Send(selRetain)
}

func releaseObject(id objc.ID) {
	if id == 0 {
		return
	}
	id.Send(selRelease)
}

type recognitionResources struct {
	recognizer objc.ID
	request    objc.ID
	task       objc.ID
	block      objc.Block
	audioFmt   objc.ID
	pcmBuf     objc.ID
}

func cleanupRecognitionResources(resources recognitionResources, endAudio bool) {
	if resources.recognizer == 0 &&
		resources.request == 0 &&
		resources.task == 0 &&
		resources.block == 0 &&
		resources.audioFmt == 0 &&
		resources.pcmBuf == 0 {
		return
	}
	done := make(chan struct{}, 1)
	SubmitToMainThread(func() {
		releaseRecognitionResources(resources, endAudio)
		done <- struct{}{}
	})
	<-done
}

func releaseRecognitionResources(resources recognitionResources, endAudio bool) {
	if endAudio && resources.request != 0 {
		resources.request.Send(selEndAudio)
	}
	if resources.task != 0 {
		resources.task.Send(selCancel)
	}
	if resources.block != 0 {
		resources.block.Release()
	}
	releaseObject(resources.task)
	releaseObject(resources.request)
	releaseObject(resources.pcmBuf)
	releaseObject(resources.audioFmt)
	releaseObject(resources.recognizer)
}

// Package-level authorization state, set by RequestSTTAuthorization() in main().
var (
	sttAuthMu     sync.Mutex
	sttAuthStatus int = -1 // -1 = not yet requested, 0..3 = SFSpeechRecognizerAuthorizationStatus
	sttAuthErr    error
)

// mainDoneCh signals RunMainRunLoop to stop.
// Buffered with capacity 1 to ensure the stop signal is not lost
// even if RunMainRunLoop is busy pumping the NSRunLoop.
var mainDoneCh = make(chan struct{}, 1)

// GCD dispatch support — used by SubmitToMainThread to dispatch closures
// to the main thread via dispatch_async_f. Works in both CLI mode
// (RunMainRunLoop pumps NSRunLoop which drains GCD main queue) and
// Tauri mode (Tauri's Cocoa event loop drains GCD main queue).
var (
	gcdOnce           sync.Once
	dispatchMainQueue uintptr // dispatch_queue_t from dispatch_get_main_queue()
	dispatchAsyncF    func(queue uintptr, context uintptr, work uintptr)
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

	if err := CheckSpeechRecognitionAccess(); err != nil {
		sttAuthErr = err
		return 0, sttAuthErr
	}

	// Check current status first
	status := objc.Send[int](objc.ID(cls), selAuthorizationStatus)
	slog.Info("[macos-stt] authorization status", "status", status)

	if status == 0 {
		if err := checkSpeechRecognitionAuthorizationRequest(); err != nil {
			sttAuthErr = err
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

	if err := CheckSpeechRecognitionAccess(); err != nil {
		sttAuthErr = err
		return 0, sttAuthErr
	}

	// Check current status
	status := objc.Send[int](objc.ID(cls), selAuthorizationStatus)
	slog.Info("[macos-stt] embedded authorization status", "status", status)

	if status == 0 {
		if err := checkSpeechRecognitionAuthorizationRequest(); err != nil {
			sttAuthErr = err
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

	deadline := timeutil.NowTime().Add(30 * time.Second)
	for {
		select {
		case status := <-authCh:
			block.Release()
			return status, nil
		default:
		}
		if timeutil.NowTime().After(deadline) {
			block.Release()
			return 0, fmt.Errorf("speech recognition authorization timed out")
		}
		// Pump run loop for 100ms
		futureDate := nsDateCls.Send(selDateWithInterval, 0.1)
		runLoop.Send(selRunUntilDate, futureDate)
	}
}

func (p *MacOSNativeSTT) Name() string               { return "macOS Native" }
func (p *MacOSNativeSTT) Type() stt.ProviderType     { return ProviderMacOSNative }
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

	// Read all audio data into memory
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	locale := langToLocale(req.Language)

	// For WAV/PCM: use buffer-based recognition (no temp file, no disk I/O)
	if req.Format == stt.FormatWAV || req.Format == stt.FormatPCM || req.Format == "" {
		if pcm, sr, ch, bps, parseErr := parseWAVData(audioData); parseErr == nil {
			text, recErr := p.recognizeFromBuffer(ctx, pcm, sr, ch, bps, locale)
			if recErr == nil {
				return &stt.TranscribeResponse{Text: text, Language: req.Language}, nil
			}
			// "No speech detected" is not a real error — return empty text
			if isNoSpeechError(recErr) {
				slog.Debug("[macos-stt] no speech detected, returning empty text")
				return &stt.TranscribeResponse{Text: "", Language: req.Language}, nil
			}
			slog.Warn("[macos-stt] buffer recognition failed, falling back to file", "error", recErr)
		}
	}

	// Fallback: write to temp file (non-WAV formats or buffer path failure)
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

	if _, err := tmpFile.Write(audioData); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write audio: %w", err)
	}
	tmpFile.Close()

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

	text, err := p.recognize(ctx, audioPath, locale)
	if err != nil {
		// "No speech detected" is not a real error — return empty text
		if isNoSpeechError(err) {
			slog.Debug("[macos-stt] no speech detected (file), returning empty text")
			return &stt.TranscribeResponse{Text: "", Language: req.Language}, nil
		}
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

// parseWAVData extracts PCM samples, sample rate, channels, and bits-per-sample from WAV data.
func parseWAVData(data []byte) (pcm []byte, sampleRate, channels, bitsPerSample int, err error) {
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, 0, 0, fmt.Errorf("not a valid WAV file")
	}
	// Parse chunks starting after "WAVE"
	offset := 12
	for offset+8 <= len(data) {
		id := string(data[offset : offset+4])
		sz := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		payload := offset + 8
		if payload+sz > len(data) {
			sz = len(data) - payload
		}
		switch id {
		case "fmt ":
			if sz < 16 {
				return nil, 0, 0, 0, fmt.Errorf("fmt chunk too small")
			}
			format := binary.LittleEndian.Uint16(data[payload : payload+2])
			if format != 1 { // PCM only
				return nil, 0, 0, 0, fmt.Errorf("unsupported WAV format: %d (need PCM=1)", format)
			}
			channels = int(binary.LittleEndian.Uint16(data[payload+2 : payload+4]))
			sampleRate = int(binary.LittleEndian.Uint32(data[payload+4 : payload+8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(data[payload+14 : payload+16]))
		case "data":
			pcm = data[payload : payload+sz]
		}
		offset += 8 + sz
		if sz%2 != 0 {
			offset++ // pad to even
		}
	}
	if pcm == nil || sampleRate == 0 {
		return nil, 0, 0, 0, fmt.Errorf("missing fmt or data chunk")
	}
	return pcm, sampleRate, channels, bitsPerSample, nil
}

// recognizeFromBuffer uses SFSpeechAudioBufferRecognitionRequest to feed PCM
// data directly to the Speech framework without writing a temp file.
func (p *MacOSNativeSTT) recognizeFromBuffer(_ context.Context, pcm []byte, sampleRate, channels, bitsPerSample int, locale string) (string, error) {
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	setupDone := make(chan struct{})
	var resources recognitionResources

	SubmitToMainThread(func() {
		defer close(setupDone)
		initSTTSelectors()
		fail := func(err error) {
			releaseRecognitionResources(resources, false)
			resources = recognitionResources{}
			ch <- result{err: err}
		}

		// Create SFSpeechRecognizer
		cls := objc.ID(objc.GetClass("SFSpeechRecognizer"))
		var nsLocale objc.ID
		if locale != "" {
			nsCls := objc.ID(objc.GetClass("NSLocale"))
			nsLocale = nsCls.Send(selAlloc).Send(selInitWithLocaleIdentifier, nsString(locale))
			resources.recognizer = cls.Send(selAlloc).Send(selInitWithLocale, nsLocale)
		} else {
			resources.recognizer = cls.Send(selAlloc).Send(selInit)
		}
		if nsLocale != 0 {
			releaseObject(nsLocale)
		}
		if resources.recognizer == 0 {
			fail(fmt.Errorf("failed to create SFSpeechRecognizer"))
			return
		}
		if !objc.Send[bool](resources.recognizer, selIsAvailable) {
			fail(fmt.Errorf("speech recognizer not available for locale %q", locale))
			return
		}

		// Create AVAudioFormat (standard float32 format)
		fmtCls := objc.ID(objc.GetClass("AVAudioFormat"))
		if fmtCls == 0 {
			fail(fmt.Errorf("AVAudioFormat class not found"))
			return
		}
		resources.audioFmt = fmtCls.Send(selAlloc).Send(selInitStdFormatSR,
			float64(sampleRate), uint32(channels))
		if resources.audioFmt == 0 {
			fail(fmt.Errorf("failed to create AVAudioFormat"))
			return
		}

		// Convert int16 PCM to float32 samples
		bytesPerSample := bitsPerSample / 8
		numSamples := len(pcm) / bytesPerSample
		frameCount := numSamples / channels

		// Create AVAudioPCMBuffer
		bufCls := objc.ID(objc.GetClass("AVAudioPCMBuffer"))
		if bufCls == 0 {
			fail(fmt.Errorf("AVAudioPCMBuffer class not found"))
			return
		}
		resources.pcmBuf = bufCls.Send(selAlloc).Send(selInitWithFormat, resources.audioFmt, uint32(frameCount))
		if resources.pcmBuf == 0 {
			fail(fmt.Errorf("failed to create AVAudioPCMBuffer"))
			return
		}

		// Set frameLength
		resources.pcmBuf.Send(selSetFrameLength, uint32(frameCount))

		// Get floatChannelData pointer: float * const *
		channelDataPtr := objc.Send[uintptr](resources.pcmBuf, selFloatChannelData)
		if channelDataPtr == 0 {
			fail(fmt.Errorf("floatChannelData returned nil"))
			return
		}

		// channelDataPtr is float**, dereference to get float* for channel 0
		ch0Ptr := *(*uintptr)(unsafe.Pointer(channelDataPtr))
		if ch0Ptr == 0 {
			fail(fmt.Errorf("channel 0 data pointer is nil"))
			return
		}

		// Convert int16 LE PCM → float32 and write into the buffer
		floatBuf := unsafe.Slice((*float32)(unsafe.Pointer(ch0Ptr)), frameCount)
		if bitsPerSample == 16 {
			for i := 0; i < frameCount; i++ {
				sampleIdx := i * channels * 2 // take first channel if stereo
				if sampleIdx+1 < len(pcm) {
					s := int16(binary.LittleEndian.Uint16(pcm[sampleIdx : sampleIdx+2]))
					floatBuf[i] = float32(s) / 32768.0
				}
			}
		}

		// Create SFSpeechAudioBufferRecognitionRequest
		reqCls := objc.ID(objc.GetClass("SFSpeechAudioBufferRecognitionRequest"))
		if reqCls == 0 {
			fail(fmt.Errorf("SFSpeechAudioBufferRecognitionRequest class not found"))
			return
		}
		resources.request = reqCls.Send(selAlloc).Send(selInit)
		if resources.request == 0 {
			fail(fmt.Errorf("failed to create buffer recognition request"))
			return
		}
		resources.request.Send(selSetShouldReportPartial, false)

		onDevice := objc.Send[bool](resources.recognizer, selSupportsOnDeviceRecognition)
		requireOnDevice := p.RequireOnDevice()
		if requireOnDevice && onDevice {
			resources.request.Send(selSetRequiresOnDevice, true)
		}
		slog.Info("[macos-stt] buffer recognition start",
			"sampleRate", sampleRate, "channels", channels, "frames", frameCount,
			"locale", locale, "onDevice", onDevice)

		// Append audio buffer and signal end
		resources.request.Send(selAppendAudioPCMBuffer, resources.pcmBuf)
		resources.request.Send(selEndAudio)

		// Result handler block
		var lastText string
		resources.block = objc.NewBlock(func(_ objc.Block, res objc.ID, nsErr objc.ID) {
			if res != 0 {
				transcription := res.Send(selBestTranscription)
				if transcription != 0 {
					text := goString(transcription.Send(selFormattedString))
					if text != "" {
						lastText = text
					}
				}
				if objc.Send[bool](res, selIsFinal) {
					select {
					case ch <- result{text: lastText}:
					default:
					}
					return
				}
			}
			if nsErr != 0 {
				desc := goString(nsErr.Send(selLocalizedDescription))
				slog.Warn("[macos-stt] buffer recognition error", "desc", desc, "lastText", lastText)
				if lastText != "" {
					select {
					case ch <- result{text: lastText}:
					default:
					}
				} else {
					var err error = fmt.Errorf("recognition error: %s", desc)
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
			}
		})

		resources.task = retainObject(resources.recognizer.Send(selRecognitionTaskWithRequest, resources.request, resources.block))
		if resources.task == 0 {
			fail(fmt.Errorf("failed to create speech recognition task"))
			return
		}

		go func() {
			time.Sleep(60 * time.Second)
			select {
			case ch <- result{err: fmt.Errorf("speech recognition timed out")}:
			default:
			}
		}()
	})
	<-setupDone

	r := <-ch
	cleanupRecognitionResources(resources, true)
	if r.err != nil {
		return "", r.err
	}
	return strings.TrimSpace(r.text), nil
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
	setupDone := make(chan struct{})
	var resources recognitionResources

	SubmitToMainThread(func() {
		defer close(setupDone)
		initSTTSelectors()
		fail := func(err error) {
			releaseRecognitionResources(resources, false)
			resources = recognitionResources{}
			ch <- result{err: err}
		}

		// Create SFSpeechRecognizer
		cls := objc.ID(objc.GetClass("SFSpeechRecognizer"))
		var nsLocale objc.ID
		if locale != "" {
			nsCls := objc.ID(objc.GetClass("NSLocale"))
			nsLocale = nsCls.Send(selAlloc).Send(selInitWithLocaleIdentifier, nsString(locale))
			resources.recognizer = cls.Send(selAlloc).Send(selInitWithLocale, nsLocale)
		} else {
			resources.recognizer = cls.Send(selAlloc).Send(selInit)
		}
		if nsLocale != 0 {
			releaseObject(nsLocale)
		}
		if resources.recognizer == 0 {
			fail(fmt.Errorf("failed to create SFSpeechRecognizer"))
			return
		}

		avail := objc.Send[bool](resources.recognizer, selIsAvailable)
		if !avail {
			fail(fmt.Errorf("speech recognizer not available for locale %q", locale))
			return
		}

		// Create NSURL from file path
		urlCls := objc.ID(objc.GetClass("NSURL"))
		audioURL := urlCls.Send(selFileURLWithPath, nsString(audioPath))
		if audioURL == 0 {
			fail(fmt.Errorf("failed to create NSURL for %s", audioPath))
			return
		}

		// Create SFSpeechURLRecognitionRequest
		reqCls := objc.ID(objc.GetClass("SFSpeechURLRecognitionRequest"))
		resources.request = reqCls.Send(selAlloc).Send(selInitWithURL, audioURL)
		if resources.request == 0 {
			fail(fmt.Errorf("failed to create recognition request"))
			return
		}
		resources.request.Send(selSetShouldReportPartial, false)

		// Check on-device support and apply user preference
		onDevice := objc.Send[bool](resources.recognizer, selSupportsOnDeviceRecognition)
		requireOnDevice := p.RequireOnDevice()
		if requireOnDevice && onDevice {
			resources.request.Send(selSetRequiresOnDevice, true)
		}
		slog.Info("[macos-stt] starting recognition task",
			"audioPath", audioPath, "locale", locale,
			"onDeviceSupported", onDevice, "requireOnDevice", requireOnDevice)

		// Block callback: ^(SFSpeechRecognitionResult *res, NSError *err)
		var lastText string
		resources.block = objc.NewBlock(func(_ objc.Block, res objc.ID, nsErr objc.ID) {
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
		resources.task = retainObject(resources.recognizer.Send(selRecognitionTaskWithRequest, resources.request, resources.block))
		if resources.task == 0 {
			fail(fmt.Errorf("failed to create speech recognition task"))
			return
		}

		// Set a timeout to release the block and send an error if no result comes
		go func() {
			time.Sleep(60 * time.Second)
			select {
			case ch <- result{err: fmt.Errorf("speech recognition timed out")}:
			default:
			}
		}()
	})
	<-setupDone

	// Wait for result from the callback (runs on this goroutine, not thread 0)
	r := <-ch
	cleanupRecognitionResources(resources, false)
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

// isNoSpeechError checks if an error represents "no speech detected" — a benign
// condition that should be treated as empty transcription, not a real error.
func isNoSpeechError(err error) bool {
	var se *SpeechError
	if errors.As(err, &se) && se.Code == "no_speech" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "no speech detected")
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

func checkSpeechRecognitionAuthorizationRequest() error {
	if err := CheckSpeechRecognitionAccess(); err != nil {
		return err
	}
	if !verifyCodeSignatureFunc() {
		slog.Warn("[macos-stt] no valid code signature")
		return fmt.Errorf("macOS native speech recognition unavailable: " +
			"no valid code signature. Sign the binary with codesign")
	}
	return nil
}

// CheckSpeechRecognitionAccess verifies that the current process declares
// speech recognition usage before touching SFSpeechRecognizer APIs. Without
// this, TCC can abort the process during even a status probe.
func CheckSpeechRecognitionAccess() error {
	initSTTSelectors()
	if !hasSpeechUsageDescriptionFunc() {
		slog.Warn("[macos-stt] NSSpeechRecognitionUsageDescription not found")
		return fmt.Errorf("macOS native speech recognition unavailable: " +
			"NSSpeechRecognitionUsageDescription missing from Info.plist")
	}
	return nil
}

// CheckMicrophoneAccess verifies that the current process declares microphone
// usage before touching AVFoundation capture authorization APIs.
func CheckMicrophoneAccess() error {
	initSTTSelectors()
	if !hasMicrophoneUsageDescriptionFunc() {
		slog.Warn("[macos-stt] NSMicrophoneUsageDescription not found")
		return fmt.Errorf("macOS native microphone unavailable: " +
			"NSMicrophoneUsageDescription missing from Info.plist")
	}
	return nil
}

// hasSpeechUsageDescription checks if NSBundle.mainBundle has the
// NSSpeechRecognitionUsageDescription key in its Info.plist.
// TCC reads this from the bundle (or embedded __TEXT,__info_plist) and will
// abort() the process if it's missing when requestAuthorization: is called.
// This preflight lets us fail gracefully instead of crashing.
func hasSpeechUsageDescription() bool {
	return hasUsageDescription("NSSpeechRecognitionUsageDescription")
}

func hasMicrophoneUsageDescription() bool {
	return hasUsageDescription("NSMicrophoneUsageDescription")
}

func hasUsageDescription(keyName string) bool {
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
	key := nsString(keyName)
	val := bundle.Send(selObjectForKey, key)
	slog.Info("[macos-stt] preflight usage description", "key", keyName, "found", val != 0)
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

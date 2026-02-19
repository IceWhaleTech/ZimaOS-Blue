//go:build darwin

package speech

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
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

// mainWorkCh receives closures to execute on thread 0's run loop.
// RunMainRunLoop drains this channel while pumping NSRunLoop.
var mainWorkCh = make(chan func(), 16)

// mainDoneCh signals RunMainRunLoop to stop.
var mainDoneCh = make(chan struct{})

// SubmitToMainThread dispatches a closure to execute on thread 0.
// Blocks until the closure is queued (not until it completes).
func SubmitToMainThread(fn func()) {
	mainWorkCh <- fn
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
		// Drain any pending work items
		for {
			select {
			case fn := <-mainWorkCh:
				fn()
			default:
				goto pump
			}
		}
	pump:
		// Pump the run loop for 50ms to let GCD/AppKit/Speech callbacks fire
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
	initialized bool
	mu          sync.Mutex
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
	return nil
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

func (p *MacOSNativeSTT) SupportedFormats() []stt.AudioFormat {
	return []stt.AudioFormat{stt.FormatWAV, stt.FormatMP3, stt.FormatFLAC, stt.FormatOGG}
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

	locale := langToLocale(req.Language)
	text, err := p.recognize(ctx, tmpFile.Name(), locale)
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
	// All ObjC/Speech framework calls must happen on thread 0 (the main thread)
	// where NSApp's run loop is being pumped by RunMainRunLoop().
	// We dispatch the recognition setup to thread 0 and wait for the result
	// on this goroutine via a channel.

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

		// Don't force on-device recognition — it can hang if the on-device model
		// for the requested locale isn't downloaded. Let the system choose the
		// best path (server or on-device). This matches hear's default behavior.
		onDevice := objc.Send[bool](recognizer, selSupportsOnDeviceRecognition)
		slog.Info("[macos-stt] starting recognition task",
			"audioPath", audioPath, "locale", locale, "onDeviceSupported", onDevice)

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
					select {
					case ch <- result{err: fmt.Errorf("recognition error: %s", desc)}:
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

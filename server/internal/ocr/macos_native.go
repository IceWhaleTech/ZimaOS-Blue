//go:build darwin

package ocr

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"go.uber.org/zap"
)

const macOSNativeEngineName = "vision/native"

var (
	visionOnce sync.Once
	visionErr  error

	macOSNativeLockOSThread   = runtime.LockOSThread
	macOSNativeUnlockOSThread = runtime.UnlockOSThread

	selAlloc                           objc.SEL
	selInit                            objc.SEL
	selRelease                         objc.SEL
	selStringWithUTF8String            objc.SEL
	selUTF8String                      objc.SEL
	selLocalizedDescription            objc.SEL
	selDataWithBytesLength             objc.SEL
	selInitWithDataOptions             objc.SEL
	selPerformRequestsError            objc.SEL
	selResults                         objc.SEL
	selTopCandidates                   objc.SEL
	selString                          objc.SEL
	selCount                           objc.SEL
	selObjectAtIndex                   objc.SEL
	selAddObject                       objc.SEL
	selSetRecognitionLanguages         objc.SEL
	selSetUsesLanguageCorrection       objc.SEL
	selSetAutomaticallyDetectsLanguage objc.SEL
)

type nativeOCRExtractor func(ctx context.Context, imagePNG []byte, recognitionLanguages []string) (string, error)

type MacOSNativeService struct {
	logger               *zap.Logger
	recognitionLanguages []string
	extract              nativeOCRExtractor
}

// Darwin compatibility: keep existing call sites stable while switching the
// implementation behind the macOS build to Vision-native OCR.
type TesseractService = MacOSNativeService

func NewTesseractService(logger *zap.Logger, cfg Config) *TesseractService {
	return NewMacOSNativeService(logger, cfg)
}

func NewMacOSNativeService(logger *zap.Logger, cfg Config) *MacOSNativeService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MacOSNativeService{
		logger:               logger,
		recognitionLanguages: visionRecognitionLanguages(cfg.PreferredModels),
		extract:              extractMacOSNativeText,
	}
}

func (s *MacOSNativeService) Close() error {
	return nil
}

func (s *MacOSNativeService) Extract(ctx context.Context, imagePNG []byte) (Result, error) {
	if s == nil {
		return Result{}, fmt.Errorf("ocr service not available")
	}
	if len(imagePNG) == 0 {
		return Result{}, fmt.Errorf("image bytes are required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	languages := append([]string(nil), s.recognitionLanguages...)
	text, err := s.extract(ctx, imagePNG, languages)
	result := Result{
		Engine: macOSNativeEngineName,
		Model:  "system",
	}
	if len(languages) > 0 {
		result.Model = strings.Join(languages, ",")
	}
	if err != nil {
		return result, err
	}
	result.Text = normalizeText(text)
	return result, nil
}

func initVisionSelectors() error {
	visionOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			visionErr = fmt.Errorf("load Foundation.framework: %w", err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/Vision.framework/Vision", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			visionErr = fmt.Errorf("load Vision.framework: %w", err)
			return
		}

		selAlloc = objc.RegisterName("alloc")
		selInit = objc.RegisterName("init")
		selRelease = objc.RegisterName("release")
		selStringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
		selUTF8String = objc.RegisterName("UTF8String")
		selLocalizedDescription = objc.RegisterName("localizedDescription")
		selDataWithBytesLength = objc.RegisterName("dataWithBytes:length:")
		selInitWithDataOptions = objc.RegisterName("initWithData:options:")
		selPerformRequestsError = objc.RegisterName("performRequests:error:")
		selResults = objc.RegisterName("results")
		selTopCandidates = objc.RegisterName("topCandidates:")
		selString = objc.RegisterName("string")
		selCount = objc.RegisterName("count")
		selObjectAtIndex = objc.RegisterName("objectAtIndex:")
		selAddObject = objc.RegisterName("addObject:")
		selSetRecognitionLanguages = objc.RegisterName("setRecognitionLanguages:")
		selSetUsesLanguageCorrection = objc.RegisterName("setUsesLanguageCorrection:")
		selSetAutomaticallyDetectsLanguage = objc.RegisterName("setAutomaticallyDetectsLanguage:")
	})
	return visionErr
}

func extractMacOSNativeText(ctx context.Context, imagePNG []byte, recognitionLanguages []string) (string, error) {
	if err := initVisionSelectors(); err != nil {
		return "", fmt.Errorf("initialize macOS OCR: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	return withMacOSNativeThreadAffinity(func() (string, error) {
		pool := newAutoreleasePool()
		if pool != 0 {
			defer releaseObject(pool)
		}

		nsDataClass := objc.ID(objc.GetClass("NSData"))
		requestClass := objc.ID(objc.GetClass("VNRecognizeTextRequest"))
		handlerClass := objc.ID(objc.GetClass("VNImageRequestHandler"))
		arrayClass := objc.ID(objc.GetClass("NSMutableArray"))
		if nsDataClass == 0 || requestClass == 0 || handlerClass == 0 || arrayClass == 0 {
			return "", fmt.Errorf("required macOS OCR classes are unavailable")
		}

		data := nsDataClass.Send(selDataWithBytesLength, unsafe.Pointer(&imagePNG[0]), uintptr(len(imagePNG)))
		if data == 0 {
			return "", fmt.Errorf("create image data for macOS OCR")
		}

		request := requestClass.Send(selAlloc).Send(selInit)
		if request == 0 {
			return "", fmt.Errorf("create Vision OCR request")
		}
		defer releaseObject(request)
		request.Send(selSetUsesLanguageCorrection, true)
		if len(recognitionLanguages) > 0 {
			languageArray := nsStringArray(recognitionLanguages)
			if languageArray != 0 {
				request.Send(selSetRecognitionLanguages, languageArray)
				releaseObject(languageArray)
			}
		} else {
			request.Send(selSetAutomaticallyDetectsLanguage, true)
		}

		requests := arrayClass.Send(selAlloc).Send(selInit)
		if requests == 0 {
			return "", fmt.Errorf("create Vision OCR request array")
		}
		defer releaseObject(requests)
		requests.Send(selAddObject, request)

		handler := handlerClass.Send(selAlloc).Send(selInitWithDataOptions, data, objc.ID(0))
		if handler == 0 {
			return "", fmt.Errorf("create Vision OCR handler")
		}
		defer releaseObject(handler)

		var nsErr objc.ID
		if !objc.Send[bool](handler, selPerformRequestsError, requests, unsafe.Pointer(&nsErr)) {
			if nsErr != 0 {
				return "", fmt.Errorf("run macOS OCR: %s", goString(nsErr.Send(selLocalizedDescription)))
			}
			return "", fmt.Errorf("run macOS OCR")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}

		results := request.Send(selResults)
		if results == 0 {
			return "", nil
		}
		count := int(objc.Send[uint64](results, selCount))
		if count <= 0 {
			return "", nil
		}

		lines := make([]string, 0, count)
		for i := 0; i < count; i++ {
			observation := results.Send(selObjectAtIndex, uintptr(i))
			if observation == 0 {
				continue
			}
			candidates := observation.Send(selTopCandidates, uintptr(1))
			if candidates == 0 || int(objc.Send[uint64](candidates, selCount)) == 0 {
				continue
			}
			candidate := candidates.Send(selObjectAtIndex, uintptr(0))
			if candidate == 0 {
				continue
			}
			text := strings.TrimSpace(goString(candidate.Send(selString)))
			if text != "" {
				lines = append(lines, text)
			}
		}
		return strings.Join(lines, "\n"), nil
	})
}

func visionRecognitionLanguages(preferredModels []string) []string {
	if len(preferredModels) == 0 {
		return nil
	}
	modelToLanguage := map[string]string{
		"eng":     "en-US",
		"chi_sim": "zh-Hans",
		"chi_tra": "zh-Hant",
		"jpn":     "ja-JP",
		"kor":     "ko-KR",
		"fra":     "fr-FR",
		"deu":     "de-DE",
		"spa":     "es-ES",
		"ita":     "it-IT",
		"por":     "pt-PT",
		"rus":     "ru-RU",
	}

	out := make([]string, 0, len(preferredModels))
	seen := make(map[string]struct{}, len(preferredModels))
	for _, raw := range preferredModels {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		language := modelToLanguage[key]
		if language == "" && strings.Contains(trimmed, "-") {
			language = trimmed
		}
		if language == "" {
			continue
		}
		if _, ok := seen[language]; ok {
			continue
		}
		seen[language] = struct{}{}
		out = append(out, language)
	}
	return out
}

func newAutoreleasePool() objc.ID {
	poolClass := objc.ID(objc.GetClass("NSAutoreleasePool"))
	if poolClass == 0 {
		return 0
	}
	return poolClass.Send(selAlloc).Send(selInit)
}

func releaseObject(id objc.ID) {
	if id == 0 {
		return
	}
	id.Send(selRelease)
}

// Objective-C autorelease pools are thread-local. Keep pool creation, native
// work, and deferred pool teardown on the same OS thread.
func withMacOSNativeThreadAffinity(fn func() (string, error)) (string, error) {
	macOSNativeLockOSThread()
	defer macOSNativeUnlockOSThread()
	return fn()
}

func setMacOSNativeThreadHooksForTest(lock func(), unlock func()) func() {
	prevLock := macOSNativeLockOSThread
	prevUnlock := macOSNativeUnlockOSThread
	if lock == nil {
		lock = func() {}
	}
	if unlock == nil {
		unlock = func() {}
	}
	macOSNativeLockOSThread = lock
	macOSNativeUnlockOSThread = unlock
	return func() {
		macOSNativeLockOSThread = prevLock
		macOSNativeUnlockOSThread = prevUnlock
	}
}

func nsString(value string) objc.ID {
	cstr := append([]byte(value), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	if cls == 0 {
		return 0
	}
	return cls.Send(selStringWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
}

func nsStringArray(values []string) objc.ID {
	arrayClass := objc.ID(objc.GetClass("NSMutableArray"))
	if arrayClass == 0 {
		return 0
	}
	array := arrayClass.Send(selAlloc).Send(selInit)
	for _, value := range values {
		if text := nsString(value); text != 0 {
			array.Send(selAddObject, text)
		}
	}
	return array
}

func goString(nsStr objc.ID) string {
	if nsStr == 0 {
		return ""
	}
	ptr := objc.Send[uintptr](nsStr, selUTF8String)
	if ptr == 0 {
		return ""
	}
	buf := make([]byte, 0, 64)
	for {
		b := *(*byte)(unsafe.Pointer(ptr))
		if b == 0 {
			break
		}
		buf = append(buf, b)
		ptr++
	}
	return string(buf)
}

//go:build darwin

package a11y

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	darwinPermissionsOnce               sync.Once
	darwinPromptBindingsOnce            sync.Once
	darwinPromptDispatchOnce            sync.Once
	darwinAccessibilityPromptMu         sync.Mutex
	darwinAccessibilityPromptIssued     bool
	axIsProcessTrusted                  func() bool
	axIsProcessTrustedWithOptions       func(options uintptr) bool
	darwinCFDictionaryCreate            func(allocator uintptr, keys uintptr, values uintptr, numValues int64, keyCallbacks uintptr, valueCallbacks uintptr) uintptr
	darwinCFBooleanTrue                 uintptr
	darwinDispatchMainQueue             uintptr
	darwinDispatchAsyncF                func(queue uintptr, context uintptr, work uintptr)
	darwinAccessibilityGrantedProbe     = darwinAccessibilityGranted
	darwinAccessibilityPromptProbe      = darwinRequestAccessibilityPrompt
	darwinAccessibilityPromptDispatch   = darwinDispatchAccessibilityPromptToMainThread
	darwinOpenAccessibilitySettingsFunc = darwinOpenAccessibilitySettings
	darwinPendingPromptWorkMu           sync.Mutex
	darwinPendingPromptWorkID           uintptr
	darwinPendingPromptWork             = make(map[uintptr]func())
)

var darwinPromptDispatchCallback = purego.NewCallback(func(ctx uintptr) {
	darwinPendingPromptWorkMu.Lock()
	fn, ok := darwinPendingPromptWork[ctx]
	if ok {
		delete(darwinPendingPromptWork, ctx)
	}
	darwinPendingPromptWorkMu.Unlock()
	if ok && fn != nil {
		fn()
	}
})

func darwinAccessibilityGranted() bool {
	darwinPermissionsOnce.Do(func() {
		handle, err := purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		darwinRegisterOptionalPuregoFunc(handle, "AXIsProcessTrusted", &axIsProcessTrusted)
		darwinRegisterOptionalPuregoFunc(handle, "AXIsProcessTrustedWithOptions", &axIsProcessTrustedWithOptions)
	})
	if axIsProcessTrusted == nil {
		return false
	}
	return axIsProcessTrusted()
}

func darwinRequestAccessibilityPrompt() bool {
	if darwinAccessibilityGranted() {
		return true
	}
	initDarwinRuntime()
	darwinPromptBindingsOnce.Do(func() {
		if darwinCoreFoundationHandle == 0 {
			return
		}
		darwinRegisterOptionalPuregoFunc(darwinCoreFoundationHandle, "CFDictionaryCreate", &darwinCFDictionaryCreate)
		symbol, err := purego.Dlsym(darwinCoreFoundationHandle, "kCFBooleanTrue")
		if err == nil && symbol != 0 {
			darwinCFBooleanTrue = *(*uintptr)(unsafe.Pointer(symbol))
		}
	})
	if axIsProcessTrustedWithOptions == nil || darwinCFDictionaryCreate == nil || darwinCFBooleanTrue == 0 {
		return false
	}
	keyRef := darwinCFStringRef("AXTrustedCheckOptionPrompt")
	if keyRef == 0 {
		return false
	}
	if darwinCFRelease != nil {
		defer darwinCFRelease(keyRef)
	}
	keys := []uintptr{keyRef}
	values := []uintptr{darwinCFBooleanTrue}
	options := darwinCFDictionaryCreate(
		0,
		uintptr(unsafe.Pointer(&keys[0])),
		uintptr(unsafe.Pointer(&values[0])),
		1,
		0,
		0,
	)
	if options == 0 {
		return false
	}
	if darwinCFRelease != nil {
		defer darwinCFRelease(options)
	}
	return axIsProcessTrustedWithOptions(options)
}

func darwinRequestAccessibilityPromptIfNeeded() {
	darwinAccessibilityPromptMu.Lock()
	if darwinAccessibilityPromptIssued {
		darwinAccessibilityPromptMu.Unlock()
		return
	}
	darwinAccessibilityPromptIssued = true
	darwinAccessibilityPromptMu.Unlock()

	if darwinAccessibilityPromptProbe != nil && darwinAccessibilityPromptDispatch != nil {
		if darwinAccessibilityPromptDispatch(func() bool {
			return darwinAccessibilityPromptProbe()
		}) {
			return
		}
	}
	if darwinOpenAccessibilitySettingsFunc != nil {
		_ = darwinOpenAccessibilitySettingsFunc()
	}
}

func darwinResetAccessibilityPromptState() {
	darwinAccessibilityPromptMu.Lock()
	darwinAccessibilityPromptIssued = false
	darwinAccessibilityPromptMu.Unlock()
}

func darwinRegisterOptionalPuregoFunc(handle uintptr, name string, out any) {
	if handle == 0 {
		return
	}
	symbol, err := purego.Dlsym(handle, name)
	if err != nil || symbol == 0 {
		return
	}
	purego.RegisterFunc(out, symbol)
}

func darwinDispatchAccessibilityPromptToMainThread(fn func() bool) bool {
	if fn == nil {
		return false
	}
	darwinPromptDispatchOnce.Do(func() {
		libdispatch, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		darwinDispatchMainQueue, err = purego.Dlsym(libdispatch, "_dispatch_main_q")
		if err != nil {
			darwinDispatchMainQueue = 0
			return
		}
		purego.RegisterLibFunc(&darwinDispatchAsyncF, libdispatch, "dispatch_async_f")
	})
	if darwinDispatchMainQueue == 0 || darwinDispatchAsyncF == nil {
		return false
	}
	darwinPendingPromptWorkMu.Lock()
	darwinPendingPromptWorkID++
	id := darwinPendingPromptWorkID
	darwinPendingPromptWork[id] = func() {
		fn()
	}
	darwinPendingPromptWorkMu.Unlock()
	darwinDispatchAsyncF(darwinDispatchMainQueue, id, darwinPromptDispatchCallback)
	return true
}

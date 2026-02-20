package ffi

import (
	"fmt"
	"unsafe"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/loader"
	"github.com/jupiterrider/ffi"
)

var (
	whisperLib ffi.Lib
	opusLib    ffi.Lib
	loaded     bool
)

// Whisper functions
var (
	whisperInitFromFileFunc ffi.Fun
	whisperFreeFunc         ffi.Fun
	whisperFullFunc         ffi.Fun
	whisperFullNSegmentsFunc ffi.Fun
	whisperFullGetSegmentTextFunc ffi.Fun
	whisperFullGetSegmentT0Func ffi.Fun
	whisperFullGetSegmentT1Func ffi.Fun
	whisperContextDefaultParamsFunc ffi.Fun
	whisperFullDefaultParamsFunc ffi.Fun
)

// Opus functions
var (
	opusDecoderCreateFunc  ffi.Fun
	opusDecoderDestroyFunc ffi.Fun
	opusDecodeFunc         ffi.Fun
)

// Load loads whisper and opus libraries
func Load(libPath string) error {
	if loaded {
		return nil
	}

	var err error
	whisperLib, err = loader.LoadLibrary(libPath, "whisper")
	if err != nil {
		return fmt.Errorf("loading whisper: %w", err)
	}

	opusLib, err = loader.LoadLibrary(libPath, "opus")
	if err != nil {
		return fmt.Errorf("loading opus: %w", err)
	}

	if err := bindWhisper(); err != nil {
		return err
	}
	if err := bindOpus(); err != nil {
		return err
	}

	loaded = true
	return nil
}

func bindWhisper() error {
	var err error
	if whisperInitFromFileFunc, err = whisperLib.Prep("whisper_init_from_file_with_params", &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer); err != nil {
		return err
	}
	if whisperFreeFunc, err = whisperLib.Prep("whisper_free", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return err
	}
	if whisperFullFunc, err = whisperLib.Prep("whisper_full", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeSint32); err != nil {
		return err
	}
	if whisperFullNSegmentsFunc, err = whisperLib.Prep("whisper_full_n_segments", &ffi.TypeSint32, &ffi.TypePointer); err != nil {
		return err
	}
	if whisperFullGetSegmentTextFunc, err = whisperLib.Prep("whisper_full_get_segment_text", &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeSint32); err != nil {
		return err
	}
	if whisperFullGetSegmentT0Func, err = whisperLib.Prep("whisper_full_get_segment_t0", &ffi.TypeSint64, &ffi.TypePointer, &ffi.TypeSint32); err != nil {
		return err
	}
	if whisperFullGetSegmentT1Func, err = whisperLib.Prep("whisper_full_get_segment_t1", &ffi.TypeSint64, &ffi.TypePointer, &ffi.TypeSint32); err != nil {
		return err
	}
	if whisperContextDefaultParamsFunc, err = whisperLib.Prep("whisper_context_default_params", &ffi.TypePointer); err != nil {
		return err
	}
	if whisperFullDefaultParamsFunc, err = whisperLib.Prep("whisper_full_default_params", &ffi.TypePointer, &ffi.TypeSint32); err != nil {
		return err
	}
	return nil
}

func bindOpus() error {
	var err error
	if opusDecoderCreateFunc, err = opusLib.Prep("opus_decoder_create", &ffi.TypePointer, &ffi.TypeSint32, &ffi.TypeSint32, &ffi.TypePointer); err != nil {
		return err
	}
	if opusDecoderDestroyFunc, err = opusLib.Prep("opus_decoder_destroy", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return err
	}
	if opusDecodeFunc, err = opusLib.Prep("opus_decode", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypeSint32, &ffi.TypeSint32); err != nil {
		return err
	}
	return nil
}

// WhisperInitFromFile initializes whisper context
func WhisperInitFromFile(path string, params uintptr) uintptr {
	cPath := append([]byte(path), 0)
	var result ffi.Arg
	whisperInitFromFileFunc.Call(unsafe.Pointer(&result), unsafe.Pointer(&cPath[0]), unsafe.Pointer(&params))
	return uintptr(result)
}

// WhisperFree frees whisper context
func WhisperFree(ctx uintptr) {
	whisperFreeFunc.Call(nil, unsafe.Pointer(&ctx))
}

// WhisperFull runs inference
func WhisperFull(ctx uintptr, params uintptr, samples []float32) int32 {
	nSamples := int32(len(samples))
	var result ffi.Arg
	whisperFullFunc.Call(unsafe.Pointer(&result), unsafe.Pointer(&ctx), unsafe.Pointer(&params), unsafe.Pointer(&samples[0]), unsafe.Pointer(&nSamples))
	return int32(result)
}

// WhisperFullNSegments returns segment count
func WhisperFullNSegments(ctx uintptr) int32 {
	var result ffi.Arg
	whisperFullNSegmentsFunc.Call(unsafe.Pointer(&result), unsafe.Pointer(&ctx))
	return int32(result)
}

// WhisperFullGetSegmentText returns segment text
func WhisperFullGetSegmentText(ctx uintptr, iSegment int32) string {
	var result ffi.Arg
	whisperFullGetSegmentTextFunc.Call(unsafe.Pointer(&result), unsafe.Pointer(&ctx), unsafe.Pointer(&iSegment))
	ptr := (*byte)(unsafe.Pointer(uintptr(result)))
	if ptr == nil {
		return ""
	}
	return cstring(ptr)
}

func cstring(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	var length int
	for {
		if *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + uintptr(length))) == 0 {
			break
		}
		length++
	}
	return string(unsafe.Slice(ptr, length))
}

func WhisperContextDefaultParams() uintptr {
	var result ffi.Arg
	whisperContextDefaultParamsFunc.Call(unsafe.Pointer(&result))
	return uintptr(result)
}

func WhisperFullDefaultParams(strategy int32) uintptr {
	var result ffi.Arg
	whisperFullDefaultParamsFunc.Call(unsafe.Pointer(&result), unsafe.Pointer(&strategy))
	return uintptr(result)
}

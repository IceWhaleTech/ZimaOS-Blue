# FFI Migration Guide

## Overview

ZimaOS-Blue supports two build modes for Whisper/Opus integration:

1. **Static Linking (Default)**: Libraries compiled into binary
2. **FFI Dynamic Loading**: Runtime library loading (smaller binary)

## Current Status

✅ Static linking - Working
✅ FFI infrastructure - Complete
⏳ FFI runtime - Needs DLL build

## FFI Architecture

```
server/internal/
├── loader/          # Cross-platform DLL loader
├── ffi/            # Whisper/Opus FFI bindings
└── stt/
    ├── whisper_cgo.go      # Static version (build tag: cgo && whisper)
    ├── whisper_ffi.go      # FFI version (build tag: !cgo || !whisper)
    └── whisper_library_manager.go
```

## Building FFI Version

### 1. Build Shared Libraries

```bash
# Build whisper.dll, opus.dll (Windows)
./scripts/build-libs.sh

# Output: libs/whisper.dll, libs/opus.dll, libs/ggml.dll
```

### 2. Build Go (Standard Binary)

```bash
cd server
go build -o blue.exe ./cmd/blue
```

### 3. Build Tauri (No Static Linking)

Update `tauri-app/src-tauri/build.rs` to remove whisper/opus linking.

## Switching Between Modes

### Use Static Linking (Current)
```bash
go build -tags whisper -buildmode=c-archive -o libblue.a ./cmd/bluelib
```

### Use FFI
```bash
go build -o blue.exe ./cmd/blue
# Place DLLs in libs/ directory
```

## Benefits of FFI

- ✅ Smaller binary size
- ✅ Runtime library updates
- ✅ CDN distribution
- ✅ No CGO complexity

## Next Steps

1. Build shared libraries with `build-libs.sh`
2. Test FFI loading
3. Implement CDN download in `whisper_library_manager.go`
4. Update CI/CD for dual-mode builds

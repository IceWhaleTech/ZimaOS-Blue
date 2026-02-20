# Quick Build Reference

## Static Build (Default)
```bash
# Build Go library
cd server
go build -tags whisper -buildmode=c-archive -o ../tauri-app/src-tauri/lib/libblue.a ./cmd/bluelib

# Build Rust
cd ../tauri-app/src-tauri
cargo build --release

# Validate
../../scripts/validate-build.sh
```

## FFI Build (Smaller Binary)
```bash
# Build shared libraries (once)
./scripts/build-libs.sh

# Build Go binary
cd server
go build -o blue ./cmd/blue

# Place DLLs in libs/ directory
```

## Switch Modes
Edit `.buildrc`:
```bash
BUILD_MODE=static  # or ffi
```

## Troubleshooting

**Missing libgomp:**
```bash
# Windows MinGW
pacman -S mingw-w64-x86_64-gcc-libs
```

**CGO errors:**
```bash
# Ensure CGO_ENABLED=1 for static mode
export CGO_ENABLED=1
```

**Missing symbols:**
```bash
# Check exports
nm tauri-app/src-tauri/lib/libblue.a | grep BlueServer
```

## Files Added
- `server/internal/loader/` - DLL loader
- `server/internal/ffi/` - FFI bindings
- `server/internal/stt/whisper_ffi.go` - FFI implementation
- `scripts/build-libs.sh` - Build shared libraries
- `docs/FFI_MIGRATION.md` - Migration guide
- `docs/BUILD_MODES.md` - Mode comparison

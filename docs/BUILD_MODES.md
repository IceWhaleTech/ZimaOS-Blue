# Build Mode Comparison

## Static Linking vs FFI Dynamic Loading

### Static Linking (Current Default)

**Pros:**
- ✅ Single binary deployment
- ✅ No runtime dependencies
- ✅ Faster startup (no DLL loading)
- ✅ Simpler distribution

**Cons:**
- ❌ Large binary size (~124MB libblue.a)
- ❌ Requires CGO + full toolchain
- ❌ Cannot update libraries without rebuild

**Build:**
```bash
go build -tags whisper -buildmode=c-archive -o libblue.a ./cmd/bluelib
```

### FFI Dynamic Loading

**Pros:**
- ✅ Smaller binary (~10-20MB)
- ✅ No CGO required
- ✅ Runtime library updates
- ✅ CDN distribution possible
- ✅ Shared libraries across apps

**Cons:**
- ❌ Requires DLL files at runtime
- ❌ Slightly slower startup
- ❌ More complex deployment

**Build:**
```bash
./scripts/build-libs.sh  # Build DLLs once
go build -o blue ./cmd/blue
```

## Size Comparison

| Component | Static | FFI |
|-----------|--------|-----|
| Go Binary | 124MB | ~15MB |
| Whisper DLL | - | ~8MB |
| Opus DLL | - | ~1MB |
| **Total** | **124MB** | **~24MB** |

## Recommendation

- **Development**: Use static (simpler)
- **Production**: Consider FFI (smaller, updatable)
- **Embedded**: Use static (single file)

## Current Status

✅ Static linking - Working
✅ FFI infrastructure - Complete
⏳ FFI runtime - Needs DLL build (`./scripts/build-libs.sh`)

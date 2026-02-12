# v0.10.4 Tauri Packaging & Network Access

## Overview

This PRD defines the packaging strategy for ZimaOS-Blue using Tauri framework, enabling out-of-the-box desktop application experience on Windows and macOS. The application will support network access, allowing users to access Echo from other devices on the same network via a shareable URL.

## Goals

1. **Cross-platform desktop app** - Package Echo as native desktop application for Windows and macOS using Tauri
2. **Out-of-the-box experience** - Users can download and run immediately without complex setup
3. **Network accessibility** - Enable access from other devices on the local network
4. **One-click URL sharing** - Provide easy-to-copy network address in the UI

## Target Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| Windows | x64 | Supported |
| Windows | ARM64 | Future |
| macOS | Apple Silicon (ARM64) | Supported |
| macOS | Intel (x64) | Supported |

## Architecture

### Technology Stack

- **Framework**: Tauri v2.x
- **Frontend**: Existing Echo web UI
- **Backend**:
  - **Windows**: Embedded Go binary as sidecar (Echo server)
  - **macOS**: Go library via CGO, statically linked into Rust binary
- **Packaging**: Platform-specific installers (.msi/.exe for Windows, .dmg/.app for macOS)

### Platform-Specific Backend Integration

#### Windows: Sidecar Approach
- Go binary compiled as standalone executable
- Bundled as Tauri sidecar resource
- Managed via process lifecycle (spawn/kill)

#### macOS: CGO Library Approach
- Go code compiled as C-compatible static library (`.a`) using `CGO_ENABLED=1`
- Rust calls Go functions via FFI (Foreign Function Interface)
- Single unified binary - no external process management needed
- Benefits:
  - Faster startup (no process spawn overhead)
  - Smaller bundle size (no duplicate runtime)
  - Better macOS integration (single process, single signature)
  - Avoids Gatekeeper delays with external binaries

### Build Optimization Strategy

#### NO UPX Compression on Binaries

**Important**: Do NOT use UPX to compress binaries before packaging.

| Approach | Startup Time | Bundle Size | Recommendation |
|----------|--------------|-------------|----------------|
| UPX compressed binary | Slow (decompression at launch) | Smaller | **NOT recommended** |
| Uncompressed binary + compressed installer | Fast | Same after install | **Recommended** |

**Rationale**:
- UPX decompresses the entire binary into memory at startup, causing noticeable delay
- macOS Gatekeeper may re-scan UPX-compressed binaries on each launch
- DMG/installer-level compression achieves similar size reduction without runtime penalty
- Users experience fast startup while still getting a small download

**Compression Strategy**:
```
Build Phase:     [Go Library] + [Rust Binary] → Uncompressed .app
Package Phase:   [.app] → [DMG with LZMA/zlib compression]
Distribution:    Compressed DMG (~40-60% size reduction)
User Install:    DMG extracts to uncompressed .app → Fast startup
```

### System Architecture

#### Windows Architecture (Sidecar)
```
┌─────────────────────────────────────────────────────────────────┐
│                     Tauri Application                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Tauri Shell                            │   │
│  │  ┌─────────────────┐    ┌─────────────────────────────┐  │   │
│  │  │   WebView       │    │    System Tray              │  │   │
│  │  │   (Echo UI)     │    │    - Status indicator       │  │   │
│  │  │                 │    │    - Quick actions          │  │   │
│  │  └─────────────────┘    └─────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                   │
│                              ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Echo Server (Sidecar Process)           │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │   │
│  │  │ HTTP Server │  │ WebSocket   │  │ Claude Code CLI │   │   │
│  │  │ :23456       │  │ Server      │  │ (Bundled)       │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘   │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### macOS Architecture (CGO Library)
```
┌─────────────────────────────────────────────────────────────────┐
│                     Tauri Application (Single Binary)            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Tauri Shell (Rust)                     │   │
│  │  ┌─────────────────┐    ┌─────────────────────────────┐  │   │
│  │  │   WebView       │    │    System Tray              │  │   │
│  │  │   (Echo UI)     │    │    - Status indicator       │  │   │
│  │  │                 │    │    - Quick actions          │  │   │
│  │  └─────────────────┘    └─────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                   │
│                              │ FFI calls (in-process)            │
│                              ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Echo Server (Linked Go Library)              │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │   │
│  │  │ HTTP Server │  │ WebSocket   │  │ Claude Code CLI │   │   │
│  │  │ :23456       │  │ Server      │  │ (Bundled)       │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘   │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Network Access Feature

### Network Address Discovery

The application will automatically detect available network interfaces and provide accessible URLs:

1. **Local address**: `http://localhost:23456`
2. **LAN address**: `http://192.168.x.x:23456` (auto-detected)
3. **Hostname**: `http://hostname.local:23456` (mDNS/Bonjour)

### UI Components

#### Top Bar Network Address Display

```
┌─────────────────────────────────────────────────────────────────┐
│  ZimaOS Blue                                                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ 🌐 Network Access: http://192.168.1.100:23456    [📋 Copy]   ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ... (rest of the UI)                                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Alternative: Homepage Network Card

```
┌─────────────────────────────────────────────────────────────────┐
│                         Welcome to Echo                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  📡 Access from Other Devices                               ││
│  │                                                              ││
│  │  Local Network:                                              ││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ http://192.168.1.100:23456                    [📋 Copy]  │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │                                                              ││
│  │  Hostname:                                                   ││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ http://my-mac.local:23456                     [📋 Copy]  │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │                                                              ││
│  │  ⓘ Make sure devices are on the same network               ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Copy to Clipboard Behavior

1. Click the copy button
2. URL is copied to system clipboard
3. Show brief toast notification: "Address copied!"
4. Button shows checkmark for 2 seconds

## API Endpoints

### Network Information

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/network/addresses` | Get all available network addresses |
| GET | `/api/v1/network/status` | Get network connectivity status |

### Response Examples

**GET /api/v1/network/addresses**
```json
{
  "local": "http://localhost:23456",
  "lan": [
    {
      "interface": "en0",
      "address": "http://192.168.1.100:23456",
      "type": "wifi"
    },
    {
      "interface": "en1",
      "address": "http://192.168.1.101:23456",
      "type": "ethernet"
    }
  ],
  "hostname": "http://my-mac.local:23456",
  "port": 23456,
  "preferred": "http://192.168.1.100:23456"
}
```

## Tauri Configuration

### macOS CGO Library Integration

#### Go Library Export (server/cmd/bluelib/exports.go)

```go
package main

import "C"
import (
    "github.com/IceWhaleTech/ZimaOS-Blue/server"
)

//export EchoStart
func EchoStart(port C.int, dataDir *C.char) C.int {
    err := server.Start(int(C.GoString(dataDir)), int(port))
    if err != nil {
        return -1
    }
    return 0
}

//export EchoStop
func EchoStop() {
    server.Stop()
}

//export EchoGetStatus
func EchoGetStatus() C.int {
    if server.IsRunning() {
        return 1
    }
    return 0
}

func main() {}
```

#### Build Commands for Go Library

```bash
# Build static library for macOS ARM64
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
  go build -buildmode=c-archive \
  -o libecho_arm64.a \
  ./server/cmd/bluelib

# Build static library for macOS x64
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
  go build -buildmode=c-archive \
  -o libecho_x64.a \
  ./server/cmd/bluelib

# Create universal binary (fat library)
lipo -create -output libecho.a libecho_arm64.a libecho_x64.a
```

#### Rust FFI Bindings (src-tauri/src/echo_ffi.rs)

```rust
use std::ffi::CString;
use std::os::raw::{c_char, c_int};

#[link(name = "echo")]
extern "C" {
    fn EchoStart(port: c_int, data_dir: *const c_char) -> c_int;
    fn EchoStop();
    fn EchoGetStatus() -> c_int;
}

pub fn start_server(port: i32, data_dir: &str) -> Result<(), String> {
    let c_data_dir = CString::new(data_dir).map_err(|e| e.to_string())?;
    let result = unsafe { EchoStart(port as c_int, c_data_dir.as_ptr()) };
    if result == 0 {
        Ok(())
    } else {
        Err("Failed to start Echo server".to_string())
    }
}

pub fn stop_server() {
    unsafe { EchoStop() };
}

pub fn is_running() -> bool {
    unsafe { EchoGetStatus() == 1 }
}
```

#### Cargo.toml Configuration

```toml
[build-dependencies]
cc = "1.0"

# Platform-specific linking
[target.'cfg(target_os = "macos")'.dependencies]
# No additional deps needed - static linking

[target.'cfg(target_os = "windows")'.dependencies]
# Windows uses sidecar approach
```

#### build.rs for macOS

```rust
fn main() {
    #[cfg(target_os = "macos")]
    {
        // Link the Go static library
        println!("cargo:rustc-link-search=native=./lib");
        println!("cargo:rustc-link-lib=static=echo");

        // Link required system frameworks
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=Security");

        // Link Go runtime dependencies
        println!("cargo:rustc-link-lib=resolv");
    }
}
```

### tauri.conf.json

```json
{
  "$schema": "https://schema.tauri.app/config/2",
  "productName": "ZimaOS Blue",
  "version": "0.10.4",
  "identifier": "com.zimaos.echo",
  "build": {
    "beforeBuildCommand": "make build-frontend",
    "beforeDevCommand": "make dev-frontend",
    "frontendDist": "../frontend/dist",
    "devUrl": "http://localhost:5173"
  },
  "bundle": {
    "active": true,
    "targets": "all",
    "icon": [
      "icons/32x32.png",
      "icons/128x128.png",
      "icons/128x128@2x.png",
      "icons/icon.icns",
      "icons/icon.ico"
    ],
    "resources": [
      "bin/*"
    ],
    "windows": {
      "certificateThumbprint": null,
      "digestAlgorithm": "sha256",
      "timestampUrl": ""
    },
    "macOS": {
      "entitlements": null,
      "minimumSystemVersion": "10.15"
    }
  },
  "app": {
    "windows": [
      {
        "title": "ZimaOS Blue",
        "width": 1200,
        "height": 800,
        "minWidth": 800,
        "minHeight": 600,
        "resizable": true,
        "fullscreen": false
      }
    ],
    "security": {
      "csp": null
    },
    "trayIcon": {
      "iconPath": "icons/tray.png",
      "iconAsTemplate": true
    }
  }
}
```

## Build & Distribution

### Build Commands

```bash
# Development
make tauri-dev

# Build for current platform
make tauri-build

# Build for specific platform
make tauri-build-windows    # Uses sidecar approach
make tauri-build-macos      # Uses CGO library approach

# Build Go library for macOS (prerequisite)
make build-echo-lib-macos

# Build for all platforms (CI/CD)
make tauri-build-all
```

### macOS DMG Compression Configuration

Configure DMG compression in `tauri.conf.json` under `bundle.macOS`:

```json
{
  "bundle": {
    "macOS": {
      "dmg": {
        "compression": "lzma"
      }
    }
  }
}
```

Or use `create-dmg` with explicit compression:

```bash
# Create compressed DMG (post-build)
create-dmg \
  --volname "ZimaOS Blue" \
  --window-pos 200 120 \
  --window-size 600 400 \
  --icon-size 100 \
  --icon "ZimaOS Blue.app" 175 120 \
  --hide-extension "ZimaOS Blue.app" \
  --app-drop-link 425 120 \
  --format ULMO \
  "ZimaOS-Blue_${VERSION}_${ARCH}.dmg" \
  "target/release/bundle/macos/ZimaOS Blue.app"
```

**DMG Format Options**:
| Format | Compression | Speed | Size Reduction |
|--------|-------------|-------|----------------|
| ULMO | LZMA | Slow | Best (~60%) |
| UDBZ | bzip2 | Medium | Good (~50%) |
| UDZO | zlib | Fast | Moderate (~40%) |

**Recommendation**: Use `ULMO` (LZMA) for release builds, `UDZO` for development.

### Output Artifacts

| Platform | Artifact | Location |
|----------|----------|----------|
| Windows | `ZimaOS-Blue_0.10.4_x64-setup.exe` | `target/release/bundle/nsis/` |
| Windows | `ZimaOS-Blue_0.10.4_x64.msi` | `target/release/bundle/msi/` |
| macOS | `ZimaOS-Blue.app` | `target/release/bundle/macos/` |
| macOS | `ZimaOS-Blue_0.10.4_aarch64.dmg` | `target/release/bundle/dmg/` |
| macOS | `ZimaOS-Blue_0.10.4_x64.dmg` | `target/release/bundle/dmg/` |

### CI/CD Pipeline

```yaml
# .github/workflows/tauri-build.yml
name: Build Tauri App

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    strategy:
      matrix:
        include:
          - platform: macos-latest
            target: aarch64-apple-darwin
          - platform: macos-latest
            target: x86_64-apple-darwin
          - platform: windows-latest
            target: x86_64-pc-windows-msvc

    runs-on: ${{ matrix.platform }}

    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Setup Rust
        uses: dtolnay/rust-toolchain@stable
        with:
          targets: ${{ matrix.target }}

      - name: Install dependencies
        run: npm ci

      - name: Build Tauri app
        uses: tauri-apps/tauri-action@v0
        with:
          tagName: v__VERSION__
          releaseName: 'ZimaOS Blue v__VERSION__'
          releaseBody: 'See the assets to download this version.'
          releaseDraft: true
          prerelease: false
```

## Implementation Checklist

### Phase 1: Tauri Setup
- [ ] Initialize Tauri project structure
- [ ] Configure tauri.conf.json
- [ ] Create application icons for all platforms

### Phase 2: macOS CGO Library Integration
- [ ] Create Go library exports (`server/cmd/bluelib/exports.go`)
- [ ] Implement `EchoStart`, `EchoStop`, `EchoGetStatus` C-exported functions
- [ ] Set up build scripts for ARM64 and x64 static libraries
- [ ] Create universal binary (fat library) with `lipo`
- [ ] Write Rust FFI bindings (`src-tauri/src/echo_ffi.rs`)
- [ ] Configure `build.rs` for static linking
- [ ] Test in-process server lifecycle

### Phase 3: Windows Sidecar Integration
- [ ] Embed Echo Go binary as Tauri sidecar
- [ ] Implement server lifecycle management (start/stop)
- [ ] Handle server port conflicts
- [ ] Add health check for embedded server

### Phase 4: Network Feature
- [ ] Implement network interface detection
- [ ] Create `/api/v1/network/addresses` endpoint
- [ ] Add network address display component to UI
- [ ] Implement copy-to-clipboard functionality
- [ ] Add toast notification for copy action

### Phase 5: UI Integration
- [ ] Add network address bar to top navigation
- [ ] Create network info card for homepage
- [ ] Add system tray with quick actions
- [ ] Implement window state persistence

### Phase 6: Build & Distribution
- [ ] Set up Makefile targets for Tauri builds
- [ ] Configure DMG compression (LZMA)
- [ ] **DO NOT use UPX** - verify no UPX in build pipeline
- [ ] Configure GitHub Actions for CI/CD
- [ ] Set up code signing for Windows
- [ ] Set up notarization for macOS
- [ ] Create release automation

### Phase 7: Testing
- [ ] Test on Windows 10/11
- [ ] Test on macOS (Intel and Apple Silicon)
- [ ] **Verify startup time < 3 seconds** (no UPX delay)
- [ ] Test network access from mobile devices
- [ ] Test installer/uninstaller flows
- [ ] Performance testing

## Security Considerations

1. **Network Binding**: By default, bind to `0.0.0.0` to allow network access, but provide option to restrict to localhost only
2. **Firewall**: Guide users to allow firewall access on first run
3. **Authentication**: Consider adding optional authentication for network access
4. **HTTPS**: Future support for self-signed certificates for secure local network access
5. **Code Signing**: Sign Windows executables and notarize macOS apps

## Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Application startup time | < 3 seconds |
| Memory usage (idle) | < 200MB |
| Installer size (Windows) | < 100MB |
| Installer size (macOS) | < 100MB |
| Network address detection | < 500ms |
| Copy to clipboard response | < 100ms |

## Future Enhancements

1. **Linux Support**: Add Linux packaging (AppImage, .deb, .rpm)
2. **Auto-update**: Integrate Tauri's built-in updater
3. **QR Code**: Generate QR code for easy mobile access
4. **mDNS Discovery**: Automatic discovery of Echo instances on network
5. **Remote Access**: Optional secure tunnel for access outside local network

## References

- [Tauri Documentation](https://tauri.app/v2/guides/)
- [Tauri GitHub Actions](https://github.com/tauri-apps/tauri-action)
- [v0.10.0 Claude Code CLI Bundling](./v0.10.0-claude-code-bundling.md)

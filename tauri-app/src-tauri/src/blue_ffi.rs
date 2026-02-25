// Blue Server FFI bindings for macOS and Windows
// Links to the Go static library (libblue.a on macOS, libblue.lib on Windows) via CGO
//
// This module provides FFI bindings to the Go server library for both platforms.

#![allow(dead_code)]

use log::{error, info};
use std::ffi::CString;
use std::os::raw::{c_char, c_int};
use std::sync::atomic::{AtomicBool, Ordering};

// FFI declarations for the Go library exports
// macOS: links to libblue.a
// Windows: links to libblue.lib
#[cfg(target_os = "macos")]
#[link(name = "blue", kind = "static")]
extern "C" {
    /// Start the Blue server with command-line arguments
    /// port: The port to listen on (0 for auto-select)
    /// data_dir: Path to the data directory (can be null for default)
    /// args: Command-line arguments as a single string (can be null)
    /// Returns: 0 on success, non-zero on error
    fn BlueServerStartWithArgs(port: c_int, data_dir: *const c_char, args: *const c_char) -> c_int;

    /// Start the Blue server (legacy, without args)
    /// port: The port to listen on (0 for auto-select)
    /// data_dir: Path to the data directory (can be null for default)
    /// Returns: 0 on success, non-zero on error
    fn BlueServerStart(port: c_int, data_dir: *const c_char) -> c_int;

    /// Stop the Blue server
    /// Returns: 0 on success, non-zero on error
    fn BlueServerStop() -> c_int;

    /// Check if the server is running
    /// Returns: 1 if running, 0 if not
    fn BlueServerIsRunning() -> c_int;

    /// Get the server version string
    /// Returns: A C string that must be freed with BlueServerFreeString
    fn BlueServerGetVersion() -> *mut c_char;

    /// Get the actual port the server is listening on
    /// Returns: port number (0 if not yet listening)
    fn BlueServerGetPort() -> c_int;

    /// Free a string returned by the Go library
    fn BlueServerFreeString(s: *mut c_char);

    /// Force cleanup of all CGO resources
    fn BlueServerCleanup();

    /// Request macOS speech recognition authorization (must be called from main thread)
    /// Returns: authorization status (0=NotDetermined, 1=Denied, 2=Restricted, 3=Authorized, -1=N/A)
    fn BlueRequestSTTAuthorization() -> c_int;
}

#[cfg(target_os = "windows")]
#[link(name = "blue", kind = "static")]
extern "C" {
    /// Start the Blue server with command-line arguments
    /// port: The port to listen on (0 for auto-select)
    /// data_dir: Path to the data directory (can be null for default)
    /// args: Command-line arguments as a single string (can be null)
    /// Returns: 0 on success, non-zero on error
    fn BlueServerStartWithArgs(port: c_int, data_dir: *const c_char, args: *const c_char) -> c_int;

    /// Start the Blue server (legacy, without args)
    /// port: The port to listen on (0 for auto-select)
    /// data_dir: Path to the data directory (can be null for default)
    /// Returns: 0 on success, non-zero on error
    fn BlueServerStart(port: c_int, data_dir: *const c_char) -> c_int;

    /// Stop the Blue server
    /// Returns: 0 on success, non-zero on error
    fn BlueServerStop() -> c_int;

    /// Check if the server is running
    /// Returns: 1 if running, 0 if not
    fn BlueServerIsRunning() -> c_int;

    /// Get the server version string
    /// Returns: A C string that must be freed with BlueServerFreeString
    fn BlueServerGetVersion() -> *mut c_char;

    /// Get the actual port the server is listening on
    /// Returns: port number (0 if not yet listening)
    fn BlueServerGetPort() -> c_int;

    /// Free a string returned by the Go library
    fn BlueServerFreeString(s: *mut c_char);

    /// Force cleanup of all CGO resources
    fn BlueServerCleanup();

    /// Request speech recognition authorization (no-op on Windows, returns -1)
    fn BlueRequestSTTAuthorization() -> c_int;
}

/// Track if we've started the server (to prevent double-start)
static SERVER_STARTED: AtomicBool = AtomicBool::new(false);

/// Start the Blue server via FFI with command-line arguments
///
/// # Arguments
/// * `port` - The port to listen on (use 0 for auto-select)
/// * `data_dir` - Optional path to the data directory
/// * `args` - Optional command-line arguments string
///
/// # Returns
/// * `Ok(())` on success
/// * `Err(String)` with error message on failure
pub fn start_server_with_args(port: u16, data_dir: Option<&str>, args: Option<&str>) -> Result<(), String> {
    if SERVER_STARTED.load(Ordering::SeqCst) {
        info!("Blue server already started via FFI");
        return Ok(());
    }

    info!("Starting Blue server via FFI on port {} with args: {:?}", port, args);

    let c_data_dir = match data_dir {
        Some(dir) => {
            CString::new(dir).map_err(|e| format!("Invalid data_dir path: {}", e))?
        }
        None => CString::new("").unwrap(),
    };

    let c_args = match args {
        Some(arg_str) => {
            CString::new(arg_str).map_err(|e| format!("Invalid args: {}", e))?
        }
        None => CString::new("").unwrap(),
    };

    let result = unsafe {
        BlueServerStartWithArgs(
            port as c_int,
            if data_dir.is_some() {
                c_data_dir.as_ptr()
            } else {
                std::ptr::null()
            },
            if args.is_some() {
                c_args.as_ptr()
            } else {
                std::ptr::null()
            },
        )
    };

    if result == 0 {
        SERVER_STARTED.store(true, Ordering::SeqCst);
        info!("Blue server started successfully via FFI with args");
        Ok(())
    } else {
        error!("Failed to start Blue server via FFI, error code: {}", result);
        Err(format!("Failed to start Blue server, error code: {}", result))
    }
}

/// Start the Blue server via FFI (legacy, without args)
pub fn start_server(port: u16, data_dir: Option<&str>) -> Result<(), String> {
    if SERVER_STARTED.load(Ordering::SeqCst) {
        info!("Blue server already started via FFI");
        return Ok(());
    }

    info!("Starting Blue server via FFI on port {}", port);

    let c_data_dir = match data_dir {
        Some(dir) => {
            CString::new(dir).map_err(|e| format!("Invalid data_dir path: {}", e))?
        }
        None => CString::new("").unwrap(),
    };

    let result = unsafe {
        BlueServerStart(
            port as c_int,
            if data_dir.is_some() {
                c_data_dir.as_ptr()
            } else {
                std::ptr::null()
            },
        )
    };

    if result == 0 {
        SERVER_STARTED.store(true, Ordering::SeqCst);
        info!("Blue server started successfully via FFI");
        Ok(())
    } else {
        error!("Failed to start Blue server via FFI, error code: {}", result);
        Err(format!("Failed to start Blue server, error code: {}", result))
    }
}

/// Stop the Blue server via FFI
pub fn stop_server() -> Result<(), String> {
    if !SERVER_STARTED.load(Ordering::SeqCst) {
        info!("Blue server not running, nothing to stop");
        return Ok(());
    }

    info!("Stopping Blue server via FFI");

    let result = unsafe { BlueServerStop() };

    if result == 0 {
        SERVER_STARTED.store(false, Ordering::SeqCst);
        info!("Blue server stopped successfully");
        Ok(())
    } else {
        error!("Failed to stop Blue server, error code: {}", result);
        Err(format!("Failed to stop Blue server, error code: {}", result))
    }
}

/// Check if the server is running
pub fn is_running() -> bool {
    unsafe { BlueServerIsRunning() == 1 }
}

/// Get the actual port the server is listening on (0 if not yet listening)
pub fn get_port() -> u16 {
    let p = unsafe { BlueServerGetPort() };
    if p > 0 { p as u16 } else { 0 }
}

/// Get the server version
pub fn get_version() -> String {
    unsafe {
        let version_ptr = BlueServerGetVersion();
        if version_ptr.is_null() {
            return "unknown".to_string();
        }

        let version = std::ffi::CStr::from_ptr(version_ptr)
            .to_string_lossy()
            .into_owned();

        BlueServerFreeString(version_ptr);
        version
    }
}

/// Force cleanup of all CGO resources
/// This should be called before process termination to ensure proper cleanup
pub fn cleanup() {
    info!("Forcing cleanup of CGO resources");
    unsafe { BlueServerCleanup() };
}

/// Request macOS speech recognition authorization via Go FFI.
/// Must be called from the main thread on macOS (Tauri's Cocoa thread).
/// Returns the authorization status:
///   -1 = not applicable (non-macOS)
///    0 = not determined
///    1 = denied
///    2 = restricted
///    3 = authorized
pub fn request_stt_authorization() -> i32 {
    let status = unsafe { BlueRequestSTTAuthorization() };
    info!("STT authorization result: {}", status);
    status as i32
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_is_running_initially_false() {
        // Server should not be running initially
        assert!(!is_running());
    }
}

// Echo Server FFI bindings for macOS
// Links to the Go static library (libecho.a) via CGO
//
// This module is only compiled on macOS where we use the CGO library approach
// instead of the sidecar process approach used on Windows.

#![cfg(target_os = "macos")]
#![allow(dead_code)]

use log::{error, info};
use std::ffi::CString;
use std::os::raw::{c_char, c_int};
use std::sync::atomic::{AtomicBool, Ordering};

// FFI declarations for the Go library exports
#[link(name = "echo", kind = "static")]
extern "C" {
    /// Start the Echo server
    /// port: The port to listen on (0 for auto-select)
    /// data_dir: Path to the data directory (can be null for default)
    /// Returns: 0 on success, non-zero on error
    fn EchoServerStart(port: c_int, data_dir: *const c_char) -> c_int;

    /// Stop the Echo server
    /// Returns: 0 on success, non-zero on error
    fn EchoServerStop() -> c_int;

    /// Check if the server is running
    /// Returns: 1 if running, 0 if not
    fn EchoServerIsRunning() -> c_int;

    /// Get the server version string
    /// Returns: A C string that must be freed with EchoServerFreeString
    fn EchoServerGetVersion() -> *mut c_char;

    /// Free a string returned by the Go library
    fn EchoServerFreeString(s: *mut c_char);
}

/// Track if we've started the server (to prevent double-start)
static SERVER_STARTED: AtomicBool = AtomicBool::new(false);

/// Start the Echo server via FFI
///
/// # Arguments
/// * `port` - The port to listen on (use 0 for auto-select)
/// * `data_dir` - Optional path to the data directory
///
/// # Returns
/// * `Ok(())` on success
/// * `Err(String)` with error message on failure
pub fn start_server(port: u16, data_dir: Option<&str>) -> Result<(), String> {
    if SERVER_STARTED.load(Ordering::SeqCst) {
        info!("Echo server already started via FFI");
        return Ok(());
    }

    info!("Starting Echo server via FFI on port {}", port);

    let c_data_dir = match data_dir {
        Some(dir) => {
            CString::new(dir).map_err(|e| format!("Invalid data_dir path: {}", e))?
        }
        None => CString::new("").unwrap(),
    };

    let result = unsafe {
        EchoServerStart(
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
        info!("Echo server started successfully via FFI");
        Ok(())
    } else {
        error!("Failed to start Echo server via FFI, error code: {}", result);
        Err(format!("Failed to start Echo server, error code: {}", result))
    }
}

/// Stop the Echo server via FFI
pub fn stop_server() -> Result<(), String> {
    if !SERVER_STARTED.load(Ordering::SeqCst) {
        info!("Echo server not running, nothing to stop");
        return Ok(());
    }

    info!("Stopping Echo server via FFI");

    let result = unsafe { EchoServerStop() };

    if result == 0 {
        SERVER_STARTED.store(false, Ordering::SeqCst);
        info!("Echo server stopped successfully");
        Ok(())
    } else {
        error!("Failed to stop Echo server, error code: {}", result);
        Err(format!("Failed to stop Echo server, error code: {}", result))
    }
}

/// Check if the server is running
pub fn is_running() -> bool {
    unsafe { EchoServerIsRunning() == 1 }
}

/// Get the server version
pub fn get_version() -> String {
    unsafe {
        let version_ptr = EchoServerGetVersion();
        if version_ptr.is_null() {
            return "unknown".to_string();
        }

        let version = std::ffi::CStr::from_ptr(version_ptr)
            .to_string_lossy()
            .into_owned();

        EchoServerFreeString(version_ptr);
        version
    }
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

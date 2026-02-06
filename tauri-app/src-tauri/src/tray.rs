// Tray icon management module
// Handles system tray functionality

// Note: These utilities are kept for future tray menu implementation
#![allow(dead_code)]

use log::info;

/// Tray menu item IDs
pub mod menu_ids {
    pub const SHOW: &str = "show";
    pub const HIDE: &str = "hide";
    pub const COPY_ADDRESS: &str = "copy_address";
    pub const SERVER_STATUS: &str = "server_status";
    pub const RESTART_SERVER: &str = "restart_server";
    pub const QUIT: &str = "quit";
}

/// Get the tooltip text for the tray icon
pub fn get_tooltip(running: bool, port: u16) -> String {
    if running {
        format!("ZimaOS Echo - Running on port {}", port)
    } else {
        "ZimaOS Echo - Server stopped".to_string()
    }
}

/// Log tray event
pub fn log_tray_event(event: &str) {
    info!("Tray event: {}", event);
}

// ZimaOS Blue - Tauri Desktop Application
// Main entry point

// Always hide console window on Windows (even in debug mode)
#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

fn main() {
    zimaos_blue_lib::run()
}

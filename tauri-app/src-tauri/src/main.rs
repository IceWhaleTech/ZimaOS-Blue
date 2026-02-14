// ZimaOS Blue - Tauri Desktop Application
// Main entry point

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    zimaos_echo_lib::run()
}

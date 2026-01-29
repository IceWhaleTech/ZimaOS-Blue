// ZimaOS Echo - Tauri Library
// Core application logic

mod server;
mod tray;

use log::{error, info};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager, RunEvent,
};

/// Application state shared across the app
pub struct AppState {
    pub server_port: std::sync::Mutex<u16>,
    pub server_running: std::sync::Mutex<bool>,
}

impl Default for AppState {
    fn default() -> Self {
        Self {
            server_port: std::sync::Mutex::new(8080),
            server_running: std::sync::Mutex::new(false),
        }
    }
}

/// Get the server URL for the current port
#[tauri::command]
fn get_server_url(state: tauri::State<AppState>) -> String {
    let port = state.server_port.lock().unwrap();
    format!("http://localhost:{}", *port)
}

/// Check if the server is running
#[tauri::command]
fn is_server_running(state: tauri::State<AppState>) -> bool {
    *state.server_running.lock().unwrap()
}

/// Get the current server port
#[tauri::command]
fn get_server_port(state: tauri::State<AppState>) -> u16 {
    *state.server_port.lock().unwrap()
}

/// Main application entry point
pub fn run() {
    // Initialize logger
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    info!("Starting ZimaOS Echo desktop application");

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_os::init())
        .manage(AppState::default())
        .invoke_handler(tauri::generate_handler![
            get_server_url,
            is_server_running,
            get_server_port,
            server::start_server,
            server::stop_server,
            server::restart_server,
            server::get_server_status,
        ])
        .setup(|app| {
            info!("Setting up application");

            // Create tray menu
            let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
            let show = MenuItem::with_id(app, "show", "Show Window", true, None::<&str>)?;
            let hide = MenuItem::with_id(app, "hide", "Hide Window", true, None::<&str>)?;
            let separator = MenuItem::with_id(app, "sep", "─────────", false, None::<&str>)?;

            let menu = Menu::with_items(app, &[&show, &hide, &separator, &quit])?;

            // Build tray icon
            let _tray = TrayIconBuilder::new()
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "quit" => {
                        info!("Quit requested from tray");
                        app.exit(0);
                    }
                    "show" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                    "hide" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.hide();
                        }
                    }
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        let app = tray.app_handle();
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                })
                .build(app)?;

            // Start the Echo server as sidecar
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                if let Err(e) = server::start_sidecar_server(&app_handle).await {
                    error!("Failed to start server: {}", e);
                }
            });

            info!("Application setup complete");
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app_handle, event| {
            if let RunEvent::ExitRequested { api, .. } = event {
                // Prevent exit, minimize to tray instead
                api.prevent_exit();
                if let Some(window) = app_handle.get_webview_window("main") {
                    let _ = window.hide();
                }
            }
        });
}

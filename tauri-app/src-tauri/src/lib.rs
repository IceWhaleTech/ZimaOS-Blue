// ZimaOS Echo - Tauri Library
// Core application logic

mod server;
mod tray;

// macOS: Use CGO library approach (FFI to Go static library)
#[cfg(target_os = "macos")]
mod echo_ffi;

use log::{error, info};
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{
    image::Image,
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager, RunEvent,
};

/// Flag to track if we're actually quitting (vs just hiding to tray)
static QUITTING: AtomicBool = AtomicBool::new(false);

/// Application state shared across the app
pub struct AppState {
    pub server_port: std::sync::Mutex<u16>,
    pub server_running: std::sync::Mutex<bool>,
}

impl Default for AppState {
    fn default() -> Self {
        Self {
            server_port: std::sync::Mutex::new(23456),
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

/// Open URL in system default browser (no ACL required)
#[tauri::command]
async fn open_url(url: String) -> Result<(), String> {
    open::that(url).map_err(|e| e.to_string())
}

/// Start the server using platform-specific approach
/// - macOS: Uses CGO library (FFI to Go static library) for faster startup
/// - Windows: Uses sidecar process
async fn start_server_platform(app: &tauri::AppHandle) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        info!("Starting Echo server via CGO library (macOS)");

        // Get data directory
        let data_dir = app.path().app_data_dir()
            .map(|p| p.to_string_lossy().to_string())
            .ok();

        // Start server via FFI
        echo_ffi::start_server(23456, data_dir.as_deref())?;

        // Update app state
        if let Some(state) = app.try_state::<AppState>() {
            *state.server_port.lock().unwrap() = 23456;
            *state.server_running.lock().unwrap() = true;
        }

        // Wait for server to be ready with exponential backoff (optimized for macOS FFI)
        let url = "http://localhost:23456/api/v1/health";
        let mut delay_ms = 25u64;  // 优化: 更快的首次检查
        let max_delay_ms = 100u64;
        let max_attempts = 8;      // 优化: 减少重试次数

        for i in 0..max_attempts {
            tokio::time::sleep(std::time::Duration::from_millis(delay_ms)).await;
            if reqwest::get(url).await.is_ok() {
                info!("Server ready after attempt {} (~{}ms total)", i + 1,
                    (0..=i).map(|j| std::cmp::min(25 * 2u64.pow(j as u32), max_delay_ms)).sum::<u64>());
                return Ok(());
            }
            delay_ms = std::cmp::min(delay_ms * 2, max_delay_ms);
        }

        info!("Server may not be fully ready, but FFI call succeeded");
        Ok(())
    }

    #[cfg(not(target_os = "macos"))]
    {
        // Windows/Linux: Use sidecar process approach
        info!("Starting Echo server via sidecar process");
        server::start_sidecar_server(app).await
    }
}

/// Stop the server using platform-specific approach
#[allow(dead_code)]
async fn stop_server_platform(app: &tauri::AppHandle) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        info!("Stopping Echo server via CGO library (macOS)");
        echo_ffi::stop_server()?;

        if let Some(state) = app.try_state::<AppState>() {
            *state.server_running.lock().unwrap() = false;
        }
        Ok(())
    }

    #[cfg(not(target_os = "macos"))]
    {
        server::stop_server(app.clone()).await
    }
}

/// Main application entry point
pub fn run() {
    // Initialize logger with optimized settings for Windows
    // Use warn level in release builds to reduce startup overhead
    #[cfg(debug_assertions)]
    let default_level = "info";
    #[cfg(not(debug_assertions))]
    let default_level = "warn";

    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or(default_level))
        .format_timestamp(None)
        .format_module_path(false)
        .format_target(false)
        .format_level(false)
        .init();

    info!("Starting ZimaOS Echo desktop application");

    #[cfg(target_os = "macos")]
    info!("Platform: macOS (using CGO library approach)");

    #[cfg(target_os = "windows")]
    info!("Platform: Windows (using sidecar approach)");

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_os::init())
        .plugin(tauri_plugin_opener::init())
        .manage(AppState::default())
        .invoke_handler(tauri::generate_handler![
            get_server_url,
            is_server_running,
            get_server_port,
            open_url,
            server::start_server,
            server::stop_server,
            server::restart_server,
            server::get_server_status,
        ])
        .setup(|app| {
            info!("Setting up application");

            // Create tray menu with improved styling
            let show = MenuItem::with_id(app, "show", "📂 Show Window", true, None::<&str>)?;
            let hide = MenuItem::with_id(app, "hide", "🙈 Hide Window", true, None::<&str>)?;
            let separator1 = MenuItem::with_id(app, "sep1", "", false, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "❌ Quit", true, None::<&str>)?;

            let menu = Menu::with_items(app, &[&show, &hide, &separator1, &quit])?;

            // Build tray icon with embedded image
            let icon = Image::from_bytes(include_bytes!("../icons/tray.png"))
                .expect("Failed to load tray icon");
            let tray = TrayIconBuilder::new()
                .icon(icon)
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "quit" => {
                        info!("Quit requested from tray");

                        // Set quitting flag so ExitRequested handler allows exit
                        QUITTING.store(true, Ordering::SeqCst);

                        // Stop server before exit on macOS
                        #[cfg(target_os = "macos")]
                        {
                            let _ = echo_ffi::stop_server();
                        }

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

            // Store tray icon in app state for cleanup on Windows
            app.manage(tray);

            // Open devtools in debug builds (must be done in setup, before async tasks)
            #[cfg(debug_assertions)]
            if let Some(window) = app.get_webview_window("main") {
                window.open_devtools();
            }

            // Start the Echo server using platform-specific approach
            let app_handle = app.handle().clone();
            let app_handle_for_window = app.handle().clone();

            // Show window immediately with loading state
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }

            tauri::async_runtime::spawn(async move {
                info!("Attempting to start server...");
                match start_server_platform(&app_handle).await {
                    Ok(_) => info!("Server started successfully"),
                    Err(e) => {
                        error!("Failed to start server: {}", e);
                        return;
                    }
                }

                // Get the server port
                let port = if let Some(state) = app_handle.try_state::<AppState>() {
                    *state.server_port.lock().unwrap()
                } else {
                    23456
                };

                // Navigate the main window to the Go server URL
                if let Some(window) = app_handle_for_window.get_webview_window("main") {
                    let url = format!("http://localhost:{}", port);
                    info!("Navigating to server at {}", url);
                    if let Err(e) = window.navigate(url.parse().unwrap()) {
                        error!("Failed to navigate to server: {}", e);
                    }

                    // On macOS, we need to activate the app to bring it to front
                    #[cfg(target_os = "macos")]
                    {
                        use std::process::Command;
                        let _ = Command::new("osascript")
                            .args(["-e", "tell application \"ZimaOS Echo\" to activate"])
                            .output();
                    }
                } else {
                    error!("Failed to get main window");
                }
            });

            info!("Application setup complete");
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app_handle, event| {
            match event {
                RunEvent::ExitRequested { api, .. } => {
                    // Only prevent exit if we're not actually quitting
                    if !QUITTING.load(Ordering::SeqCst) {
                        api.prevent_exit();
                        if let Some(window) = app_handle.get_webview_window("main") {
                            let _ = window.hide();
                        }
                    }
                }
                RunEvent::Exit => {
                    // Clean up tray icon on Windows to prevent ghost icons
                    #[cfg(target_os = "windows")]
                    {
                        if let Some(tray) = app_handle.try_state::<tauri::tray::TrayIcon>() {
                            let _ = tray.set_visible(false);
                        }
                    }
                }
                _ => {}
            }
        });
}

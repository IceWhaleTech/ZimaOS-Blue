// ZimaOS Blue - Tauri Library
// Core application logic

mod server;
mod tray;

#[cfg(target_os = "windows")]
mod windows_service;

// macOS & Windows: Use CGO library approach (FFI to Go static library)
#[cfg(any(target_os = "macos", target_os = "windows"))]
mod blue_ffi;

use clap::Parser;
use log::{error, info};
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{
    image::Image,
    menu::{Menu, MenuItem},
    tray::{MouseButton, TrayIconBuilder, TrayIconEvent},
    Manager, RunEvent,
};

/// ZimaOS Blue - A Local-first Agent Runtime
#[derive(Parser, Debug, Clone, Default)]
#[command(name = "blue", version, about = "ZimaOS Blue desktop application")]
pub struct CliArgs {
    /// Server listen port
    #[arg(short, long)]
    port: Option<u16>,

    /// Config file path
    #[arg(long)]
    config: Option<String>,

    /// Data directory path
    #[arg(long)]
    data_dir: Option<String>,

    /// Enable dev mode (isolate state under ~/.zimaos-blue-dev)
    #[arg(long)]
    dev: bool,

    /// Verbose logging (debug level)
    #[arg(short, long)]
    verbose: bool,
}

/// Flag to track if we're actually quitting (vs just hiding to tray)
static QUITTING: AtomicBool = AtomicBool::new(false);

/// Close behavior: false = quit, true = minimize to tray
static MINIMIZE_TO_TRAY: AtomicBool = AtomicBool::new(false);

/// Stores the tray quit MenuItem so we can update its text dynamically
struct TrayQuitItem(MenuItem<tauri::Wry>);

/// Map locale string to localized "Quit ZimaOS Blue" label
fn quit_label_for_locale(locale: &str) -> String {
    let prefix = locale.split(&['-', '_'][..]).next().unwrap_or("en");
    let verb = match prefix {
        "zh" => "退出",
        "ja" => "終了",
        "ko" => "종료",
        "fr" => "Quitter",
        "de" => "Beenden",
        "es" => "Salir",
        "it" => "Esci",
        "pt" => "Sair",
        "nl" => "Afsluiten",
        "ca" => "Sortir",
        "sv" => "Avsluta",
        "da" => "Afslut",
        "nb" | "no" => "Avslutt",
        "ga" => "Scoir",
        "pl" => "Zakończ",
        "cs" => "Ukončit",
        "sk" => "Ukončiť",
        "hu" => "Kilépés",
        "ro" => "Ieșire",
        "hr" => "Izlaz",
        "el" => "Έξοδος",
        "ru" => "Выход",
        "ml" => "പുറത്തുകടക്കുക",
        _ => "Quit",
    };
    format!("{} ZimaOS Blue", verb)
}

/// Gracefully shut down: close connections, stop Go server, then exit.
/// We avoid NSApplication.terminate() because it triggers NSPersistentUIManager's
/// synchronous XPC flush, which races with Go runtime cleanup and causes SIGABRT.
#[cfg(any(target_os = "macos", target_os = "windows"))]
fn graceful_quit(app_handle: &tauri::AppHandle) {
    if QUITTING.swap(true, Ordering::SeqCst) {
        return; // Already quitting
    }

    // 1. Destroy all windows first — this closes SSE/WebSocket connections
    //    so the HTTP server can shut down without waiting for idle connections.
    info!("Graceful quit: closing windows");
    for (_, window) in app_handle.webview_windows() {
        let _ = window.destroy();
    }

    // 2. Stop the Go server (blocks until server is down or timeout).
    //    Internally this cancels all active SSE streams, closes WebSocket
    //    connections, then shuts down the HTTP server, and performs cleanup.
    info!("Graceful quit: stopping Go server");
    let _ = blue_ffi::stop_server();

    // 3. Final CGo resource cleanup (already done in BlueServerStop, but call again for safety)
    blue_ffi::cleanup();

    // 4. Exit via Tauri's event loop — this fires RunEvent::Exit for
    //    platform cleanup (e.g. tray icon removal on Windows), then exits.
    info!("Graceful quit: exiting");
    app_handle.exit(0);
}
pub struct AppState {
    pub server_port: std::sync::Mutex<u16>,
    pub server_running: std::sync::Mutex<bool>,
    pub cli_args: CliArgs,
}

impl Default for AppState {
    fn default() -> Self {
        Self {
            server_port: std::sync::Mutex::new(80),
            server_running: std::sync::Mutex::new(false),
            cli_args: CliArgs::default(),
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

/// Update tray menu language to match app locale
#[tauri::command]
fn set_tray_locale(app: tauri::AppHandle, locale: String) {
    let label = quit_label_for_locale(&locale);
    if let Some(quit_item) = app.try_state::<TrayQuitItem>() {
        let _ = quit_item.0.set_text(&label);
        info!("Tray menu language updated to: {} ({})", locale, label);
    }
}

/// Set close behavior: "quit" or "minimize"
#[tauri::command]
fn set_close_behavior(behavior: String) {
    let minimize = behavior == "minimize";
    MINIMIZE_TO_TRAY.store(minimize, Ordering::SeqCst);
    info!("Close behavior set to: {}", behavior);
}

/// Install Windows service
#[tauri::command]
fn install_windows_service() -> Result<String, String> {
    #[cfg(target_os = "windows")]
    {
        windows_service::install_service()
    }
    #[cfg(not(target_os = "windows"))]
    {
        Err("Service installation only supported on Windows".to_string())
    }
}

/// Uninstall Windows service
#[tauri::command]
fn uninstall_windows_service() -> Result<String, String> {
    #[cfg(target_os = "windows")]
    {
        windows_service::uninstall_service()
    }
    #[cfg(not(target_os = "windows"))]
    {
        Err("Service uninstallation only supported on Windows".to_string())
    }
}

/// Start server with command-line arguments (macOS specific)
#[tauri::command]
async fn start_server_with_args(app: tauri::AppHandle, args: Option<String>) -> Result<(), String> {
    start_server_platform_with_args(&app, args).await
}

/// Start the server using platform-specific approach with optional command-line arguments
/// - macOS & Windows: Uses CGO library (FFI to Go static library) for faster startup
async fn start_server_platform_with_args(app: &tauri::AppHandle, args: Option<String>) -> Result<(), String> {
    // Merge explicit args with CLI args from AppState
    let cli_args_str = if let Some(state) = app.try_state::<AppState>() {
        build_args_string(&state.cli_args, args.as_deref())
    } else {
        args.clone()
    };

    // Determine port: explicit CLI --port > default 80
    let port = if let Some(state) = app.try_state::<AppState>() {
        state.cli_args.port.unwrap_or(80)
    } else {
        80
    };

    #[cfg(any(target_os = "macos", target_os = "windows"))]
    {
        info!("Starting Blue server via CGO library with args: {:?}", cli_args_str);

        // Get data directory: CLI --data-dir > default
        let data_dir = if let Some(state) = app.try_state::<AppState>() {
            state.cli_args.data_dir.clone()
        } else {
            None
        }.unwrap_or_else(|| {
            dirs::home_dir()
                .map(|h| h.join(".zimaos-blue").to_string_lossy().to_string())
                .unwrap_or_else(|| ".zimaos-blue".to_string())
        });

        blue_ffi::start_server_with_args(port, Some(&data_dir), cli_args_str.as_deref())?;

        if let Some(state) = app.try_state::<AppState>() {
            *state.server_port.lock().unwrap() = port;
            *state.server_running.lock().unwrap() = true;
        }

        let url = format!("http://localhost:{}/api/v1/health", port);
        let mut delay_ms = 25u64;
        let max_delay_ms = 100u64;
        let max_attempts = 8;

        for i in 0..max_attempts {
            tokio::time::sleep(std::time::Duration::from_millis(delay_ms)).await;
            if reqwest::get(&url).await.is_ok() {
                info!("Server ready after attempt {} with args", i + 1);
                return Ok(());
            }
            delay_ms = std::cmp::min(delay_ms * 2, max_delay_ms);
        }

        info!("Server may not be fully ready, but FFI call succeeded");
        Ok(())
    }

    #[cfg(target_os = "linux")]
    {
        info!("Starting Blue server via sidecar process with args: {:?}", cli_args_str);
        server::start_sidecar_server_with_args(app, cli_args_str.as_deref()).await
    }
}

/// Build a combined args string from CliArgs and optional extra args
fn build_args_string(cli: &CliArgs, extra: Option<&str>) -> Option<String> {
    let mut parts: Vec<String> = Vec::new();

    if let Some(p) = cli.port {
        parts.push(format!("--port {}", p));
    }
    if let Some(ref c) = cli.config {
        parts.push(format!("--config {}", c));
    }
    if let Some(ref d) = cli.data_dir {
        parts.push(format!("--data-dir {}", d));
    }
    if cli.dev {
        parts.push("--dev".to_string());
    }
    if cli.verbose {
        parts.push("--verbose".to_string());
    }
    if let Some(extra) = extra {
        if !extra.is_empty() {
            parts.push(extra.to_string());
        }
    }

    if parts.is_empty() {
        None
    } else {
        Some(parts.join(" "))
    }
}

/// Start the server using platform-specific approach
#[allow(dead_code)]
async fn start_server_platform(app: &tauri::AppHandle) -> Result<(), String> {
    start_server_platform_with_args(app, None).await
}

/// Stop the server using platform-specific approach
#[allow(dead_code)]
async fn stop_server_platform(app: &tauri::AppHandle) -> Result<(), String> {
    #[cfg(any(target_os = "macos", target_os = "windows"))]
    {
        info!("Stopping Blue server via CGO library");
        blue_ffi::stop_server()?;

        if let Some(state) = app.try_state::<AppState>() {
            *state.server_running.lock().unwrap() = false;
        }
        Ok(())
    }

    #[cfg(target_os = "linux")]
    {
        server::stop_server(app.clone()).await
    }
}

/// Main application entry point
pub fn run() {
    // Parse CLI arguments
    let cli_args = CliArgs::parse();

    // Initialize logger with optimized settings for Windows
    // Use warn level in release builds to reduce startup overhead
    #[cfg(debug_assertions)]
    let default_level = if cli_args.verbose { "debug" } else { "info" };
    #[cfg(not(debug_assertions))]
    let default_level = if cli_args.verbose { "debug" } else { "warn" };

    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or(default_level))
        .format_timestamp(None)
        .format_module_path(false)
        .format_target(false)
        .format_level(false)
        .init();

    info!("Starting ZimaOS Blue desktop application");
    if cli_args.port.is_some() || cli_args.config.is_some() || cli_args.data_dir.is_some() || cli_args.dev || cli_args.verbose {
        info!("CLI args: {:?}", cli_args);
    }

    #[cfg(target_os = "macos")]
    info!("Platform: macOS (using CGO library approach)");

    #[cfg(target_os = "windows")]
    info!("Platform: Windows (using CGO library approach)");

    let default_port = cli_args.port.unwrap_or(80);
    let app_state = AppState {
        server_port: std::sync::Mutex::new(default_port),
        server_running: std::sync::Mutex::new(false),
        cli_args,
    };

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // When second instance is launched, show and focus the first instance
            info!("Second instance detected, focusing existing window");
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
                let _ = window.unminimize();
            }
        }))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_os::init())
        .plugin(tauri_plugin_opener::init())
        .manage(app_state)
        .invoke_handler(tauri::generate_handler![
            get_server_url,
            is_server_running,
            get_server_port,
            open_url,
            set_close_behavior,
            install_windows_service,
            uninstall_windows_service,
            set_tray_locale,
            start_server_with_args,
            server::start_server,
            server::stop_server,
            server::restart_server,
            server::get_server_status,
        ])
        .on_page_load(|webview, _payload| {
            // Inject Tauri marker into external pages (Go server at localhost)
            // so the frontend can detect it's running inside Tauri webview.
            // Without this, __TAURI_INTERNALS__ is only available on tauri:// URLs.
            let _ = webview.eval(
                "if(!window.__TAURI_INTERNALS__){window.__TAURI_INTERNALS__={__desktop:true}}"
            );
        })
        .setup(|app| {
            info!("Setting up application");

            // Detect system language for initial tray menu (updated dynamically after webview loads)
            let quit_label = {
                let lang = sys_locale::get_locale().unwrap_or_else(|| "en".to_string());
                quit_label_for_locale(&lang)
            };

            // Create tray menu — quit only (right-click menu)
            let quit = MenuItem::with_id(app, "quit", quit_label, true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&quit])?;

            // Build tray icon with template image (macOS auto-adapts for light/dark mode)
            let icon = Image::from_bytes(include_bytes!("../icons/tray.png"))
                .expect("Failed to load tray icon");
            let tray = TrayIconBuilder::new()
                .icon(icon)
                .icon_as_template(true)
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "quit" => {
                        info!("Quit requested from tray");
                        #[cfg(any(target_os = "macos", target_os = "windows"))]
                        graceful_quit(&app.app_handle());
                        #[cfg(target_os = "linux")]
                        {
                            QUITTING.store(true, Ordering::SeqCst);
                            info!("Stopping server before exit");
                            #[cfg(target_os = "linux")]
                            {
                                let app_clone = app.app_handle().clone();
                                tauri::async_runtime::block_on(async {
                                    let _ = server::stop_server(app_clone).await;
                                });
                            }
                            app.exit(0);
                        }
                    }
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        ..
                    } = event
                    {
                        #[cfg(target_os = "macos")]
                        {
                            use objc2::MainThreadMarker;
                            use objc2_app_kit::{NSApplication, NSApplicationActivationPolicy};
                            if let Some(mtm) = MainThreadMarker::new() {
                                let ns_app = NSApplication::sharedApplication(mtm);
                                ns_app.setActivationPolicy(NSApplicationActivationPolicy::Regular);
                            }
                        }
                        let app = tray.app_handle();
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        } else {
                            // Window was destroyed — recreate it
                            let port = if let Some(state) = app.try_state::<AppState>() {
                                *state.server_port.lock().unwrap()
                            } else {
                                80
                            };
                            let url = format!("http://localhost:{}", port);
                            if let Ok(window) = tauri::WebviewWindowBuilder::new(
                                app,
                                "main",
                                tauri::WebviewUrl::External(url.parse().unwrap()),
                            )
                            .title("ZimaOS Blue")
                            .inner_size(1400.0, 900.0)
                            .min_inner_size(800.0, 600.0)
                            .center()
                            .visible(false) // Start hidden to prevent flash
                            .build()
                            {
                                // Window will be shown by frontend after content loads
                                let _ = window.set_focus();
                            }
                        }
                    }
                })
                .build(app)?;

            // Store tray icon in app state for cleanup on Windows
            app.manage(tray);
            app.manage(TrayQuitItem(quit));

            // Open devtools in debug builds (must be done in setup, before async tasks)
            #[cfg(debug_assertions)]
            if let Some(window) = app.get_webview_window("main") {
                window.open_devtools();
            }

            // Start the Blue server using platform-specific approach
            let app_handle = app.handle().clone();
            let app_handle_for_window = app.handle().clone();

            // On macOS, request speech recognition authorization from the main thread
            // before starting the Go server. TCC requires this to happen on thread 0
            // where the Cocoa event loop runs.
            #[cfg(target_os = "macos")]
            {
                info!("Requesting macOS speech recognition authorization...");
                let status = blue_ffi::request_stt_authorization();
                match status {
                    3 => info!("Speech recognition authorized"),
                    1 => info!("Speech recognition denied by user"),
                    2 => info!("Speech recognition restricted"),
                    0 => info!("Speech recognition not determined"),
                    _ => info!("Speech recognition status: {}", status),
                }
            }

            tauri::async_runtime::spawn(async move {
                info!("Attempting to start server...");
                match start_server_platform(&app_handle).await {
                    Ok(_) => info!("Server started successfully"),
                    Err(e) => {
                        error!("Failed to start server: {}", e);
                        // Show error dialog to user
                        #[cfg(not(target_os = "linux"))]
                        {
                            use tauri::Manager;
                            if let Some(window) = app_handle.get_webview_window("main") {
                                let _ = window.eval(&format!(
                                    "alert('Failed to start server: {}');",
                                    e.replace("'", "\\'")
                                ));
                            }
                        }
                        return;
                    }
                }

                // Get the server port
                let port = if let Some(state) = app_handle.try_state::<AppState>() {
                    *state.server_port.lock().unwrap()
                } else {
                    80
                };

                // Navigate the main window to the Go server URL
                if let Some(window) = app_handle_for_window.get_webview_window("main") {
                    let url = format!("http://localhost:{}", port);
                    info!("Navigating to server at {}", url);
                    if let Err(e) = window.navigate(url.parse().unwrap()) {
                        error!("Failed to navigate to server: {}", e);
                    }

                    // Wait briefly for the page to start loading before showing window
                    tokio::time::sleep(std::time::Duration::from_millis(300)).await;

                    // Now show the window after navigation has started
                    let _ = window.show();
                    let _ = window.set_focus();

                    // On macOS, we need to activate the app to bring it to front
                    #[cfg(target_os = "macos")]
                    {
                        use std::process::Command;
                        let _ = Command::new("osascript")
                            .args(["-e", "tell application \"ZimaOS Blue\" to activate"])
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
                    // Only prevent exit if we're not actually quitting AND minimize-to-tray is enabled
                    if !QUITTING.load(Ordering::SeqCst) && MINIMIZE_TO_TRAY.load(Ordering::SeqCst) {
                        api.prevent_exit();
                        if let Some(window) = app_handle.get_webview_window("main") {
                            let _ = window.hide();
                        }
                        // On macOS, hide the Dock icon when minimizing to tray
                        #[cfg(target_os = "macos")]
                        {
                            use objc2::MainThreadMarker;
                            use objc2_app_kit::{NSApplication, NSApplicationActivationPolicy};
                            if let Some(mtm) = MainThreadMarker::new() {
                                let ns_app = NSApplication::sharedApplication(mtm);
                                ns_app.setActivationPolicy(NSApplicationActivationPolicy::Accessory);
                            }
                        }
                    } else if !QUITTING.load(Ordering::SeqCst) {
                        // "quit" behavior: stop server and exit
                        #[cfg(any(target_os = "macos", target_os = "windows"))]
                        graceful_quit(app_handle);
                        #[cfg(target_os = "linux")]
                        {
                            QUITTING.store(true, Ordering::SeqCst);
                            info!("Graceful quit: stopping server");
                            #[cfg(target_os = "linux")]
                            {
                                let app_clone = app_handle.clone();
                                tauri::async_runtime::block_on(async {
                                    let _ = server::stop_server(app_clone).await;
                                });
                            }
                            app_handle.exit(0);
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

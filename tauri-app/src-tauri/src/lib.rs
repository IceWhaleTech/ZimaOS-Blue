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
use std::path::{Path, PathBuf};
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

const MAIN_WINDOW_LABEL: &str = "main";
const MAIN_WINDOW_TITLE: &str = "ZimaOS Blue";
const MAIN_WINDOW_WIDTH: f64 = 1400.0;
const MAIN_WINDOW_HEIGHT: f64 = 900.0;
const MAIN_WINDOW_MIN_WIDTH: f64 = 800.0;
const MAIN_WINDOW_MIN_HEIGHT: f64 = 600.0;
const ABOUT_BLANK_SPLASH_SCRIPT: &str = r#"
document.documentElement.style.background = 'transparent';
document.body.style.cssText = 'margin:0;background:transparent;display:flex;align-items:center;justify-content:center;height:100vh';
document.body.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;width:96px;height:96px;border-radius:28px;background:rgba(15,23,42,0.34);box-shadow:inset 0 1px 0 rgba(255,255,255,0.14),0 24px 60px rgba(2,6,23,0.22);backdrop-filter:blur(26px) saturate(1.12);-webkit-backdrop-filter:blur(26px) saturate(1.12)"><div style="width:36px;height:36px;border:3px solid rgba(148,163,184,0.72);border-top-color:#3B82F6;border-radius:50%;animation:s .8s linear infinite"></div></div><style>@keyframes s{to{transform:rotate(360deg)}}@media(prefers-color-scheme:light){body>div{background:rgba(255,255,255,0.52)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,0.68),0 24px 60px rgba(148,163,184,0.22)!important}body>div>div{border-color:rgba(100,116,139,0.56)!important;border-top-color:#3B82F6!important}}</style>';
"#;

#[cfg(target_os = "macos")]
const MACOS_GLASS_INIT_SCRIPT: &str = r#"
(() => {
  const isLocalDesktopPage =
    window.location.href === 'about:blank' ||
    window.location.hostname === 'localhost' ||
    window.location.hostname === '127.0.0.1';
  if (!isLocalDesktopPage) return;
  window.__BLUE_DESKTOP__ = true;
  window.__BLUE_MACOS_GLASS__ = true;
})();
"#;

/// Stores the tray quit MenuItem so we can update its text dynamically
struct TrayQuitItem(MenuItem<tauri::Wry>);

#[cfg(target_os = "macos")]
fn main_window_effects() -> tauri::utils::config::WindowEffectsConfig {
    use tauri::{
        utils::config::WindowEffectsConfig,
        window::{Effect, EffectState},
    };

    WindowEffectsConfig {
        effects: vec![Effect::HudWindow],
        state: Some(EffectState::FollowsWindowActiveState),
        radius: Some(18.0),
        color: None,
    }
}

fn build_main_window<R: tauri::Runtime, M: Manager<R>>(
    manager: &M,
    url: tauri::WebviewUrl,
) -> tauri::Result<tauri::WebviewWindow<R>> {
    let builder = tauri::WebviewWindowBuilder::new(manager, MAIN_WINDOW_LABEL, url)
        .title(MAIN_WINDOW_TITLE)
        .inner_size(MAIN_WINDOW_WIDTH, MAIN_WINDOW_HEIGHT)
        .min_inner_size(MAIN_WINDOW_MIN_WIDTH, MAIN_WINDOW_MIN_HEIGHT)
        .center()
        .visible(false);

    #[cfg(target_os = "macos")]
    let builder = builder
        .transparent(true)
        .title_bar_style(tauri::TitleBarStyle::Transparent)
        .hidden_title(true)
        .accept_first_mouse(true)
        .effects(main_window_effects())
        .initialization_script(MACOS_GLASS_INIT_SCRIPT);

    builder.build()
}

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
    pub use_https: std::sync::Mutex<bool>, // Track if server is using HTTPS
}

impl Default for AppState {
    fn default() -> Self {
        Self {
            server_port: std::sync::Mutex::new(80),
            server_running: std::sync::Mutex::new(false),
            cli_args: CliArgs::default(),
            use_https: std::sync::Mutex::new(false),
        }
    }
}

fn parse_bool_env_flag(raw: &str) -> Option<bool> {
    match raw.trim().to_ascii_lowercase().as_str() {
        "1" | "true" | "yes" | "on" => Some(true),
        "0" | "false" | "no" | "off" => Some(false),
        _ => None,
    }
}

fn stt_auth_startup_enabled(
    debug_build: bool,
    cli_dev_mode: bool,
    env_override: Option<&str>,
) -> bool {
    if let Some(raw) = env_override {
        if let Some(value) = parse_bool_env_flag(raw) {
            return value;
        }
    }

    !(debug_build || cli_dev_mode)
}

#[cfg(target_os = "macos")]
fn should_request_stt_authorization_on_startup(cli_args: &CliArgs) -> bool {
    let env_override = std::env::var("ZIMAOS_STT_AUTH_ON_STARTUP").ok();
    if let Some(raw) = env_override.as_deref() {
        if parse_bool_env_flag(raw).is_none() {
            info!(
                "Ignoring invalid ZIMAOS_STT_AUTH_ON_STARTUP value {:?}; expected one of 1/0/true/false/yes/no/on/off",
                raw
            );
        }
    }

    stt_auth_startup_enabled(
        cfg!(debug_assertions),
        cli_args.dev,
        env_override.as_deref(),
    )
}

/// Get the server URL for the current port
#[tauri::command]
fn get_server_url(state: tauri::State<AppState>) -> String {
    let port = state.server_port.lock().unwrap();
    let use_https = state.use_https.lock().unwrap();
    let protocol = if *use_https { "https" } else { "http" };
    format!("{}://localhost:{}", protocol, *port)
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

fn resolve_reveal_target(raw_path: &str) -> Result<(std::path::PathBuf, bool), String> {
    let trimmed = raw_path.trim();
    if trimmed.is_empty() {
        return Err("path is required".to_string());
    }
    let lower = trimmed.to_ascii_lowercase();
    if lower.starts_with("http://")
        || lower.starts_with("https://")
        || lower.starts_with("/api/")
        || trimmed.contains("://")
    {
        return Err("path must be a local absolute filesystem path".to_string());
    }

    let path = std::path::PathBuf::from(trimmed);
    if !path.is_absolute() {
        return Err("path must be absolute".to_string());
    }
    let metadata = std::fs::metadata(&path).map_err(|e| format!("path not accessible: {}", e))?;
    Ok((path, metadata.is_dir()))
}

fn parent_directory_for_reveal_fallback(path: &Path, is_dir: bool) -> Option<PathBuf> {
    if is_dir {
        return None;
    }
    let parent = path.parent()?;
    if parent == path {
        return None;
    }
    Some(parent.to_path_buf())
}

fn reveal_path_with_fallback<F>(target: &Path, is_dir: bool, reveal: F) -> Result<(), String>
where
    F: Fn(&Path, bool) -> Result<(), String>,
{
    if let Err(err) = reveal(target, is_dir) {
        if let Some(parent) = parent_directory_for_reveal_fallback(target, is_dir) {
            return reveal(&parent, true).map_err(|parent_err| {
                format!("{err} (fallback to parent directory failed: {parent_err})")
            });
        }
        return Err(err);
    }

    Ok(())
}

#[cfg(target_os = "macos")]
fn reveal_path_macos(path: &std::path::Path, is_dir: bool) -> Result<(), String> {
    if is_dir {
        return open::that(path).map_err(|e| e.to_string());
    }
    let status = std::process::Command::new("open")
        .arg("-R")
        .arg(path)
        .status()
        .map_err(|e| format!("failed to invoke Finder: {}", e))?;
    if status.success() {
        Ok(())
    } else {
        Err(format!("failed to reveal path in Finder: {}", status))
    }
}

#[cfg(target_os = "linux")]
fn reveal_path_linux(path: &std::path::Path, is_dir: bool) -> Result<(), String> {
    let target = if is_dir {
        path.to_path_buf()
    } else {
        path.parent().unwrap_or(path).to_path_buf()
    };

    match std::process::Command::new("xdg-open").arg(&target).spawn() {
        Ok(_) => Ok(()),
        Err(_) => open::that(&target).map_err(|e| e.to_string()),
    }
}

#[cfg(target_os = "windows")]
fn path_to_wide_null(path: &std::path::Path) -> Vec<u16> {
    use std::os::windows::ffi::OsStrExt;
    path.as_os_str()
        .encode_wide()
        .chain(std::iter::once(0))
        .collect()
}

#[cfg(target_os = "windows")]
fn reveal_path_windows(path: &std::path::Path, is_dir: bool) -> Result<(), String> {
    use std::ptr::{null, null_mut};
    use windows_sys::Win32::Foundation::RPC_E_CHANGED_MODE;
    use windows_sys::Win32::System::Com::{
        CoInitializeEx, CoTaskMemFree, CoUninitialize, COINIT_APARTMENTTHREADED,
    };
    use windows_sys::Win32::UI::Shell::{
        SHOpenFolderAndSelectItems, SHParseDisplayName, ITEMIDLIST,
    };

    unsafe {
        let mut should_uninitialize = false;
        let hr_init = CoInitializeEx(null_mut(), COINIT_APARTMENTTHREADED);
        if hr_init >= 0 {
            should_uninitialize = true;
        } else if hr_init != RPC_E_CHANGED_MODE {
            return Err(format!(
                "failed to initialize COM: 0x{:08X}",
                hr_init as u32
            ));
        }

        let mut item_pidl: *mut ITEMIDLIST = null_mut();
        let result = (|| -> Result<(), String> {
            let path_wide = path_to_wide_null(path);
            let hr_parse = SHParseDisplayName(
                path_wide.as_ptr(),
                null_mut(),
                &mut item_pidl,
                0,
                null_mut(),
            );
            if hr_parse < 0 || item_pidl.is_null() {
                return Err(format!("failed to parse path: 0x{:08X}", hr_parse as u32));
            }

            if is_dir {
                let hr = SHOpenFolderAndSelectItems(item_pidl as *const ITEMIDLIST, 0, null(), 0);
                if hr < 0 {
                    return Err(format!(
                        "failed to open directory in Explorer: 0x{:08X}",
                        hr as u32
                    ));
                }
                return Ok(());
            }

            let parent = path
                .parent()
                .ok_or_else(|| "file path has no parent directory".to_string())?;
            let mut folder_pidl: *mut ITEMIDLIST = null_mut();
            let parent_wide = path_to_wide_null(parent);
            let hr_parent = SHParseDisplayName(
                parent_wide.as_ptr(),
                null_mut(),
                &mut folder_pidl,
                0,
                null_mut(),
            );
            if hr_parent < 0 || folder_pidl.is_null() {
                return Err(format!(
                    "failed to parse parent directory: 0x{:08X}",
                    hr_parent as u32
                ));
            }

            let selected: [*const ITEMIDLIST; 1] = [item_pidl as *const ITEMIDLIST];
            let hr_open = SHOpenFolderAndSelectItems(
                folder_pidl as *const ITEMIDLIST,
                1,
                selected.as_ptr(),
                0,
            );
            CoTaskMemFree(folder_pidl as *const _);
            if hr_open < 0 {
                return Err(format!(
                    "failed to reveal file in Explorer: 0x{:08X}",
                    hr_open as u32
                ));
            }
            Ok(())
        })();

        if !item_pidl.is_null() {
            CoTaskMemFree(item_pidl as *const _);
        }
        if should_uninitialize {
            CoUninitialize();
        }

        result
    }
}

/// Reveal a local absolute path in system file manager.
/// - macOS: reveal file in Finder, or open directory.
/// - Windows: COM-based Explorer selection/open.
/// - Linux: xdg-open target directory.
#[tauri::command]
async fn reveal_path(path: String) -> Result<(), String> {
    let (target, is_dir) = resolve_reveal_target(&path)?;

    reveal_path_with_fallback(&target, is_dir, |path, is_dir| {
        #[cfg(target_os = "macos")]
        {
            return reveal_path_macos(path, is_dir);
        }
        #[cfg(target_os = "windows")]
        {
            return reveal_path_windows(path, is_dir);
        }
        #[cfg(target_os = "linux")]
        {
            return reveal_path_linux(path, is_dir);
        }
        #[cfg(not(any(target_os = "macos", target_os = "windows", target_os = "linux")))]
        {
            let _ = (path, is_dir);
            Err("reveal_path is not supported on this platform".to_string())
        }
    })
}

#[cfg(test)]
mod tests {
    use super::{
        parent_directory_for_reveal_fallback, parse_bool_env_flag, reveal_path_with_fallback,
        stt_auth_startup_enabled,
    };
    use std::path::{Path, PathBuf};
    use std::sync::{Arc, Mutex};

    #[test]
    fn parse_bool_env_flag_recognizes_common_truthy_and_falsy_values() {
        assert_eq!(parse_bool_env_flag("1"), Some(true));
        assert_eq!(parse_bool_env_flag("true"), Some(true));
        assert_eq!(parse_bool_env_flag("YES"), Some(true));
        assert_eq!(parse_bool_env_flag("on"), Some(true));
        assert_eq!(parse_bool_env_flag("0"), Some(false));
        assert_eq!(parse_bool_env_flag("false"), Some(false));
        assert_eq!(parse_bool_env_flag("No"), Some(false));
        assert_eq!(parse_bool_env_flag("off"), Some(false));
        assert_eq!(parse_bool_env_flag("maybe"), None);
    }

    #[test]
    fn stt_auth_startup_enabled_defaults_to_disabled_for_debug_or_dev_mode() {
        assert!(!stt_auth_startup_enabled(true, false, None));
        assert!(!stt_auth_startup_enabled(false, true, None));
        assert!(stt_auth_startup_enabled(false, false, None));
    }

    #[test]
    fn stt_auth_startup_enabled_honors_environment_override() {
        assert!(stt_auth_startup_enabled(true, true, Some("1")));
        assert!(!stt_auth_startup_enabled(false, false, Some("0")));
        assert!(stt_auth_startup_enabled(false, false, Some("invalid")));
    }

    #[test]
    fn parent_directory_fallback_returns_parent_for_file_path() {
        let file = std::env::temp_dir().join("report.txt");
        let expected = file.parent().map(|path| path.to_path_buf());
        assert_eq!(parent_directory_for_reveal_fallback(&file, false), expected);
    }

    #[test]
    fn parent_directory_fallback_skips_directory_paths() {
        let dir = std::env::temp_dir();
        assert_eq!(parent_directory_for_reveal_fallback(&dir, true), None);
    }

    #[test]
    fn parent_directory_fallback_skips_root_paths() {
        let root = std::env::temp_dir()
            .ancestors()
            .last()
            .expect("temp dir should have a filesystem root")
            .to_path_buf();
        assert_eq!(parent_directory_for_reveal_fallback(&root, false), None);
    }

    #[test]
    fn reveal_path_with_fallback_uses_parent_directory_when_file_reveal_fails() {
        let dir = std::env::temp_dir();
        let file = dir.join("report.txt");
        let calls = Arc::new(Mutex::new(Vec::<(PathBuf, bool)>::new()));
        let recorded_calls = Arc::clone(&calls);
        let dir_for_closure = dir.clone();
        let file_for_closure = file.clone();

        let result = reveal_path_with_fallback(&file, false, move |path: &Path, is_dir| {
            recorded_calls
                .lock()
                .expect("calls lock poisoned")
                .push((path.to_path_buf(), is_dir));
            if path == file_for_closure.as_path() {
                return Err("direct file reveal unsupported".to_string());
            }
            assert_eq!(path, dir_for_closure.as_path());
            assert!(is_dir);
            Ok(())
        });

        assert_eq!(result, Ok(()));
        assert_eq!(
            calls.lock().expect("calls lock poisoned").clone(),
            vec![(file.clone(), false), (dir.clone(), true)]
        );
    }

    #[test]
    fn reveal_path_with_fallback_returns_combined_error_when_parent_reveal_fails() {
        let dir = std::env::temp_dir();
        let file = dir.join("report.txt");

        let result = reveal_path_with_fallback(&file, false, |path: &Path, is_dir| {
            if path == file.as_path() {
                return Err("direct file reveal unsupported".to_string());
            }
            assert_eq!(path, dir.as_path());
            assert!(is_dir);
            Err("parent reveal failed".to_string())
        });

        let err = result.expect_err("fallback reveal should fail");
        assert!(err.contains("direct file reveal unsupported"));
        assert!(err.contains("parent reveal failed"));
    }
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
async fn start_server_platform_with_args(
    app: &tauri::AppHandle,
    args: Option<String>,
) -> Result<(), String> {
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
        info!(
            "Starting Blue server via CGO library with args: {:?}",
            cli_args_str
        );

        // Get data directory: CLI --data-dir > default
        let data_dir = if let Some(state) = app.try_state::<AppState>() {
            state.cli_args.data_dir.clone()
        } else {
            None
        }
        .unwrap_or_else(|| {
            // For Tauri app, use data directory next to executable
            std::env::current_exe()
                .ok()
                .and_then(|exe| exe.parent().map(|p| p.join("data")))
                .and_then(|p| p.to_str().map(|s| s.to_string()))
                .unwrap_or_else(|| {
                    // Fallback to home directory if exe path fails
                    dirs::home_dir()
                        .map(|h| h.join(".zimaos-blue").to_string_lossy().to_string())
                        .unwrap_or_else(|| ".zimaos-blue".to_string())
                })
        });

        blue_ffi::start_server_with_args(port, Some(&data_dir), cli_args_str.as_deref())?;

        if let Some(state) = app.try_state::<AppState>() {
            *state.server_running.lock().unwrap() = true;
        }

        // Wait for Go server to be ready using two-phase detection:
        // Phase 1: Poll BlueServerIsRunning() + BlueServerGetPort() via FFI.
        //          Wait until the Go goroutine has started AND bound a port.
        // Phase 2: HTTP health check to confirm the listener is accepting connections.
        let mut use_https = false;
        let mut actual_port: u16 = port;

        // Phase 1: FFI poll — wait for is_running + port > 0
        let mut phase1_ok = false;
        for _ in 0..400 {
            if blue_ffi::is_running() {
                let p = blue_ffi::get_port();
                if p > 0 {
                    actual_port = p;
                    phase1_ok = true;
                    break;
                }
            }
            tokio::time::sleep(std::time::Duration::from_millis(5)).await;
        }

        if !phase1_ok {
            return Err("Server failed to start: timed out waiting for port binding".to_string());
        }

        info!("Server bound to port {}", actual_port);

        // Update state with actual port
        if let Some(state) = app.try_state::<AppState>() {
            *state.server_port.lock().unwrap() = actual_port;
        }

        // Phase 2: HTTP health check — server goroutine is running, listener may be up
        let http_url = format!("http://localhost:{}/api/v1/health", actual_port);
        let client = reqwest::Client::builder()
            .danger_accept_invalid_certs(true)
            .timeout(std::time::Duration::from_millis(500))
            .build()
            .unwrap_or_else(|_| reqwest::Client::new());

        let mut server_ready = false;
        let mut delay_ms = 10u64;
        for i in 0..20 {
            if i > 0 {
                tokio::time::sleep(std::time::Duration::from_millis(delay_ms)).await;
                delay_ms = std::cmp::min(delay_ms * 2, 200);
            }

            if client.get(&http_url).send().await.is_ok() {
                info!("Server ready (HTTP) after attempt {}", i + 1);
                server_ready = true;
                break;
            }

            // Only try HTTPS after a few HTTP failures (rare case: TLS enabled)
            if i >= 5 {
                let https_url = format!("https://localhost:{}/api/v1/health", actual_port);
                if client.get(&https_url).send().await.is_ok() {
                    info!("Server ready (HTTPS) after attempt {}", i + 1);
                    use_https = true;
                    server_ready = true;
                    break;
                }
            }
        }

        if !server_ready {
            return Err(format!(
                "Server started on port {} but health check failed after all retries",
                actual_port
            ));
        }

        // Update state with detected protocol
        if let Some(state) = app.try_state::<AppState>() {
            *state.use_https.lock().unwrap() = use_https;
        }

        info!(
            "Server protocol detected: {}",
            if use_https { "HTTPS" } else { "HTTP" }
        );
        Ok(())
    }

    #[cfg(target_os = "linux")]
    {
        info!(
            "Starting Blue server via sidecar process with args: {:?}",
            cli_args_str
        );
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

/// Listen to the Go server's SSE event stream and fire native OS notifications
/// when a `push` event arrives. Reconnects automatically on disconnect.
/// The SSE endpoint requires auth — if unauthenticated, the server-side
/// push.Notifier already handles native notifications as the primary channel.
async fn listen_push_sse(app: tauri::AppHandle, port: u16, use_https: bool) {
    use tauri_plugin_notification::NotificationExt;

    let protocol = if use_https { "https" } else { "http" };
    let url = format!("{}://localhost:{}/api/v1/events", protocol, port);

    let client = reqwest::Client::builder()
        .danger_accept_invalid_certs(true)
        .no_proxy()
        .build()
        .unwrap_or_else(|_| reqwest::Client::new());

    loop {
        info!("Connecting to SSE push stream at {}", url);
        let resp = client.get(&url).send().await;
        match resp {
            Ok(resp) if resp.status().is_success() => {
                use futures_util::StreamExt;

                let mut event_type = String::new();
                let mut data_buf = String::new();
                let mut leftover = String::new();
                let mut stream = resp.bytes_stream();

                while let Some(chunk) = stream.next().await {
                    let chunk = match chunk {
                        Ok(c) => c,
                        Err(e) => {
                            info!("SSE stream error: {}", e);
                            break;
                        }
                    };

                    leftover.push_str(&String::from_utf8_lossy(&chunk));

                    while let Some(pos) = leftover.find('\n') {
                        let line = leftover[..pos].trim_end_matches('\r').to_string();
                        leftover = leftover[pos + 1..].to_string();

                        if line.is_empty() {
                            if event_type == "push" && !data_buf.is_empty() {
                                if let Ok(val) =
                                    serde_json::from_str::<serde_json::Value>(&data_buf)
                                {
                                    let title = val["title"].as_str().unwrap_or("Blue");
                                    let body = val["message"].as_str().unwrap_or("");
                                    if !body.is_empty() {
                                        let _ = app
                                            .notification()
                                            .builder()
                                            .title(title)
                                            .body(body)
                                            .show();
                                    }
                                }
                            }
                            event_type.clear();
                            data_buf.clear();
                        } else if let Some(rest) = line.strip_prefix("event:") {
                            event_type = rest.trim().to_string();
                        } else if let Some(rest) = line.strip_prefix("data:") {
                            if !data_buf.is_empty() {
                                data_buf.push('\n');
                            }
                            data_buf.push_str(rest.trim());
                        }
                    }
                }
            }
            Ok(resp) => {
                info!(
                    "SSE push stream returned {}, server-side notifier handles push instead",
                    resp.status()
                );
            }
            Err(e) => {
                info!("SSE push connection failed: {}", e);
            }
        }

        tokio::time::sleep(std::time::Duration::from_secs(5)).await;
    }
}

/// Send a native OS notification via tauri-plugin-notification.
/// Called from the frontend when an SSE push event arrives.
#[tauri::command]
fn send_notification(app: tauri::AppHandle, title: String, body: String) -> Result<(), String> {
    use tauri_plugin_notification::NotificationExt;
    app.notification()
        .builder()
        .title(&title)
        .body(&body)
        .show()
        .map_err(|e| e.to_string())
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
    if cli_args.port.is_some()
        || cli_args.config.is_some()
        || cli_args.data_dir.is_some()
        || cli_args.dev
        || cli_args.verbose
    {
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
        use_https: std::sync::Mutex::new(false),
    };

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // When second instance is launched, show and focus the first instance
            info!("Second instance detected, focusing existing window");
            if let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) {
                let _ = window.show();
                let _ = window.set_focus();
                let _ = window.unminimize();
            }
        }))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_os::init())
        .plugin(tauri_plugin_opener::init())
        .manage(app_state)
        .invoke_handler(tauri::generate_handler![
            get_server_url,
            is_server_running,
            get_server_port,
            open_url,
            reveal_path,
            set_close_behavior,
            install_windows_service,
            uninstall_windows_service,
            set_tray_locale,
            start_server_with_args,
            send_notification,
            server::start_server,
            server::stop_server,
            server::restart_server,
            server::get_server_status,
        ])
        .on_page_load(|webview, payload| {
            let url = payload.url().to_string();

            // For about:blank, inject a splash spinner and show the window immediately.
            // This gives instant visual feedback while the Go server boots.
            if url == "about:blank" {
                let _ = webview.eval(ABOUT_BLANK_SPLASH_SCRIPT);
                let _ = webview.window().show();
                let _ = webview.window().set_focus();
            }

            // Show the window when the localhost page loads.
            if url.contains("localhost") {
                let _ = webview.window().show();
                let _ = webview.window().set_focus();
            }

            // Inject desktop marker into external pages (Go server at localhost)
            // so the frontend knows it's running inside the desktop app.
            // NOTE: We do NOT inject __TAURI_INTERNALS__ because the page is loaded
            // from an external origin (http://localhost) where Tauri IPC is unavailable.
            // The frontend should use relative URLs (same-origin) for all API calls.
            if url.contains("localhost") {
                #[cfg(target_os = "macos")]
                let _ = webview.eval(
                    "window.__BLUE_DESKTOP__=true;\
                     window.__BLUE_MACOS_GLASS__=true;\
                     document.documentElement.dataset.blueDesktop='true';\
                     document.documentElement.dataset.blueMacosGlass='true';"
                );

                #[cfg(not(target_os = "macos"))]
                let _ = webview.eval(
                    "window.__BLUE_DESKTOP__=true;\
                     document.documentElement.dataset.blueDesktop='true';"
                );
            }
        })
        .setup(|app| {
            info!("Setting up application");

            #[cfg(target_os = "macos")]
            {
                if let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) {
                    info!("Rebuilding main window with macOS vibrancy styling");
                    let _ = window.destroy();
                }

                let _ = build_main_window(
                    app.handle(),
                    tauri::WebviewUrl::CustomProtocol(
                        "about:blank"
                            .parse()
                            .expect("about:blank should be a valid URL"),
                    ),
                )?;
            }

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
                        if let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) {
                            let _ = window.show();
                            let _ = window.set_focus();
                        } else {
                            // Window was destroyed — recreate it
                            let (port, use_https) = if let Some(state) = app.try_state::<AppState>() {
                                (
                                    *state.server_port.lock().unwrap(),
                                    *state.use_https.lock().unwrap(),
                                )
                            } else {
                                (80, false)
                            };
                            let protocol = if use_https { "https" } else { "http" };
                            let url = format!("{}://localhost:{}", protocol, port);
                            if let Ok(window) = build_main_window(
                                app,
                                tauri::WebviewUrl::External(url.parse().unwrap()),
                            ) {
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
            if let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) {
                window.open_devtools();
            }

            // Start the Blue server using platform-specific approach
            let app_handle = app.handle().clone();
            let app_handle_for_window = app.handle().clone();

            // On macOS, request speech recognition authorization AFTER NSApp is fully
            // launched. TCC requires this on thread 0 with the Cocoa event loop running.
            // Calling it during setup (before NSApp finishes launching) causes abort().
            // Use run_on_main_thread to defer until the run loop is active.
            #[cfg(target_os = "macos")]
            {
                let should_request_stt_auth = app
                    .try_state::<AppState>()
                    .map(|state| should_request_stt_authorization_on_startup(&state.cli_args))
                    .unwrap_or_else(|| {
                        should_request_stt_authorization_on_startup(&CliArgs::default())
                    });

                if should_request_stt_auth {
                    let stt_app_handle = app.handle().clone();
                    let _ = stt_app_handle.run_on_main_thread(move || {
                        info!("Requesting macOS speech recognition authorization...");
                        let status = blue_ffi::request_stt_authorization();
                        match status {
                            3 => info!("Speech recognition authorized"),
                            1 => info!("Speech recognition denied by user"),
                            2 => info!("Speech recognition restricted"),
                            0 => info!("Speech recognition not determined"),
                            _ => info!("Speech recognition status: {}", status),
                        }
                    });
                } else {
                    info!(
                        "Skipping automatic macOS speech recognition authorization during startup. \
Set ZIMAOS_STT_AUTH_ON_STARTUP=1 to force it in debug/dev runs."
                    );
                }
            }

            tauri::async_runtime::spawn(async move {
                info!("Attempting to start server...");
                match start_server_platform(&app_handle).await {
                    Ok(_) => info!("Server started successfully"),
                    Err(e) => {
                        error!("Failed to start server: {}", e);
                        // Show error page in the webview
                        if let Some(window) = app_handle.get_webview_window(MAIN_WINDOW_LABEL) {
                            let escaped = e.replace('\\', "\\\\").replace('\'', "\\'").replace('\n', "\\n");
                            let _ = window.eval(&format!(
                                "document.documentElement.style.background='transparent';\
                                 document.body.style.cssText='margin:0;background:transparent;color:#F8FAFC;display:flex;flex-direction:column;align-items:center;justify-content:center;height:100vh;font-family:system-ui,sans-serif';\
                                 document.body.innerHTML='<div style=\"text-align:center;max-width:520px;margin:16px;padding:2rem;border-radius:28px;background:rgba(15,23,42,0.68);box-shadow:inset 0 1px 0 rgba(255,255,255,0.12),0 32px 80px rgba(2,6,23,0.3);backdrop-filter:blur(28px) saturate(1.12);-webkit-backdrop-filter:blur(28px) saturate(1.12)\">\
                                 <div style=\"font-size:48px;margin-bottom:16px\">&#9888;&#65039;</div>\
                                 <h2 style=\"margin:0 0 12px;font-size:20px\">Server Failed to Start</h2>\
                                 <p style=\"color:#94A3B8;font-size:14px;line-height:1.6;margin:0 0 24px\">{}</p>\
                                 <button onclick=\"location.reload()\" style=\"background:#3B82F6;color:#fff;border:none;padding:10px 24px;border-radius:8px;font-size:14px;cursor:pointer\">Retry</button>\
                                 </div>';\
                                 document.querySelector(\"style\")?.remove();",
                                escaped
                            ));
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                        return;
                    }
                }

                // Get the server port and protocol
                let (port, use_https) = if let Some(state) = app_handle.try_state::<AppState>() {
                    (
                        *state.server_port.lock().unwrap(),
                        *state.use_https.lock().unwrap(),
                    )
                } else {
                    (80, false)
                };

                // Start SSE listener for push notifications (native OS notifications)
                {
                    let sse_port = port;
                    let sse_https = use_https;
                    let sse_app = app_handle.clone();
                    tokio::spawn(async move {
                        listen_push_sse(sse_app, sse_port, sse_https).await;
                    });
                }

                // Navigate the main window to the Go server URL
                if let Some(window) = app_handle_for_window.get_webview_window(MAIN_WINDOW_LABEL) {
                    let protocol = if use_https { "https" } else { "http" };
                    let url = format!("{}://localhost:{}", protocol, port);
                    info!("Navigating to server at {}", url);
                    if let Err(e) = window.navigate(url.parse().unwrap()) {
                        error!("Failed to navigate to server: {}", e);
                    }

                    // on_page_load shows the window as soon as the HTML loads (splash visible).
                    // Fallback: if frontend somehow fails, force-show after 5s.
                    tokio::time::sleep(std::time::Duration::from_millis(5000)).await;
                    if !window.is_visible().unwrap_or(true) {
                        info!("Fallback: showing window after timeout");
                        let _ = window.show();
                        let _ = window.set_focus();
                    }

                    // On macOS, activate the app to bring it to front
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
                        if let Some(window) = app_handle.get_webview_window(MAIN_WINDOW_LABEL) {
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

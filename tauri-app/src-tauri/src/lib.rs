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
use log::{error, info, warn};
use once_cell::sync::Lazy;
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

#[cfg(target_os = "macos")]
unsafe extern "C" {
    fn getppid() -> i32;
}

/// Flag to track if we're actually quitting (vs just hiding to tray/menu bar)
static QUITTING: AtomicBool = AtomicBool::new(false);

/// Close behavior: false = quit, true = minimize to tray/menu bar
static MINIMIZE_TO_TRAY: AtomicBool = AtomicBool::new(true);

struct DesktopStartupTrace {
    enabled: bool,
    component: &'static str,
    started: std::time::Instant,
    last: std::sync::Mutex<std::time::Instant>,
}

impl DesktopStartupTrace {
    fn new(component: &'static str) -> Self {
        let now = std::time::Instant::now();
        Self {
            enabled: startup_trace_env_enabled(),
            component,
            started: now,
            last: std::sync::Mutex::new(now),
        }
    }

    fn mark(&self, label: &str) {
        if !self.enabled {
            return;
        }

        let now = std::time::Instant::now();
        let mut last = self.last.lock().unwrap();
        let step = now.duration_since(*last);
        *last = now;
        let total = now.duration_since(self.started);

        let message = format!(
            "startup-trace component={} label={} step_ms={} total_ms={}",
            self.component,
            label,
            step.as_millis(),
            total.as_millis()
        );

        // Mirror startup marks to stderr so profiling runs still capture them
        // even if the structured logger output is filtered or delayed.
        eprintln!("{message}");
        info!("{message}");
    }
}

static DESKTOP_STARTUP_TRACE: Lazy<DesktopStartupTrace> =
    Lazy::new(|| DesktopStartupTrace::new("tauri.desktop"));

const MAIN_WINDOW_LABEL: &str = "main";
const MAIN_WINDOW_TITLE: &str = "ZimaOS Blue";
const MAIN_WINDOW_WIDTH: f64 = 1400.0;
const MAIN_WINDOW_HEIGHT: f64 = 900.0;
const MAIN_WINDOW_MIN_WIDTH: f64 = 800.0;
const MAIN_WINDOW_MIN_HEIGHT: f64 = 600.0;
const PANEL_WINDOW_LABEL: &str = "panel";
const PANEL_WINDOW_TITLE: &str = "ZimaOS Blue Quick Panel";
const PANEL_WINDOW_WIDTH: f64 = 880.0;
const PANEL_WINDOW_HEIGHT: f64 = 640.0;
const PANEL_WINDOW_MIN_WIDTH: f64 = 640.0;
const PANEL_WINDOW_MIN_HEIGHT: f64 = 420.0;
const PANEL_WINDOW_PATH: &str = "/chat?panel=1";
const ABOUT_BLANK_SPLASH_SCRIPT: &str = r##"
(() => {
  if (window.location.href !== 'about:blank') {
    return;
  }
  const body = document.body;
  if (!body) {
    return;
  }

  const sidebarRows = Array.from(
    { length: 5 },
    (_, index) =>
      `<span class="startup-shell__sidebar-row startup-shell__skeleton" style="--row-width:${88 - index * 10}%"></span>`
  ).join('');
  const assistantLines = Array.from(
    { length: 3 },
    (_, index) =>
      `<span class="startup-shell__line startup-shell__skeleton" style="--line-width:${96 - index * 14}%"></span>`
  ).join('');

  document.documentElement.style.background = 'transparent';
  body.style.cssText = 'margin:0;min-height:100vh;background:transparent;';
  body.innerHTML = `
    <div class="startup-shell" aria-hidden="true">
      <div class="startup-shell__backdrop"></div>
      <div class="startup-shell__layout">
        <aside class="startup-shell__sidebar">
          <div class="startup-shell__brand">
            <span class="startup-shell__brand-mark" aria-hidden="true"></span>
            <span class="startup-shell__brand-bar startup-shell__skeleton"></span>
          </div>
          <div class="startup-shell__sidebar-card">
            <span class="startup-shell__sidebar-title startup-shell__skeleton"></span>
            ${sidebarRows}
          </div>
        </aside>
        <main class="startup-shell__main">
          <header class="startup-shell__topbar">
            <span class="startup-shell__chip startup-shell__skeleton"></span>
            <span class="startup-shell__status-wrap startup-shell__status-wrap--icon-only">
              <span class="startup-shell__spinner" aria-hidden="true"></span>
            </span>
          </header>
          <section class="startup-shell__hero">
            <span class="startup-shell__hero-kicker startup-shell__skeleton"></span>
            <span class="startup-shell__hero-title startup-shell__skeleton"></span>
            <span class="startup-shell__hero-copy startup-shell__skeleton"></span>
            <span class="startup-shell__hero-copy startup-shell__skeleton startup-shell__hero-copy--short"></span>
          </section>
          <section class="startup-shell__surface">
            <article class="startup-shell__message startup-shell__message--assistant">
              <span class="startup-shell__avatar startup-shell__avatar--assistant" aria-hidden="true"></span>
              <div class="startup-shell__message-body">
                ${assistantLines}
              </div>
            </article>
            <article class="startup-shell__message startup-shell__message--user">
              <div class="startup-shell__message-body startup-shell__message-body--user">
                <span class="startup-shell__line startup-shell__skeleton" style="--line-width:58%"></span>
              </div>
            </article>
            <div class="startup-shell__composer">
              <span class="startup-shell__composer-bar startup-shell__skeleton"></span>
              <div class="startup-shell__composer-actions">
                <span class="startup-shell__composer-pill startup-shell__skeleton"></span>
                <span class="startup-shell__composer-pill startup-shell__skeleton"></span>
                <span class="startup-shell__composer-send" aria-hidden="true"></span>
              </div>
              <span class="startup-shell__composer-copy startup-shell__skeleton"></span>
            </div>
          </section>
        </main>
      </div>
    </div>
    <style>
      .startup-shell,
      .startup-shell * {
        box-sizing: border-box;
      }
      .startup-shell {
        position: relative;
        min-height: 100vh;
        overflow: hidden;
        font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
        color: rgba(241, 245, 249, 0.96);
        background:
          radial-gradient(circle at top left, rgba(56, 189, 248, 0.18), transparent 34%),
          radial-gradient(circle at top right, rgba(34, 197, 94, 0.12), transparent 30%),
          linear-gradient(180deg, rgba(8, 15, 30, 0.94), rgba(6, 12, 24, 0.98));
      }
      .startup-shell__backdrop {
        position: absolute;
        inset: -12%;
        background:
          radial-gradient(circle, rgba(59, 130, 246, 0.18), transparent 40%),
          radial-gradient(circle at 80% 20%, rgba(16, 185, 129, 0.16), transparent 32%);
        filter: blur(56px);
        opacity: 0.8;
        pointer-events: none;
      }
      .startup-shell__layout {
        position: relative;
        z-index: 1;
        min-height: 100vh;
        padding: 28px;
        display: grid;
        grid-template-columns: minmax(220px, 280px) minmax(0, 1fr);
        gap: 24px;
      }
      .startup-shell__sidebar,
      .startup-shell__surface {
        backdrop-filter: blur(28px) saturate(1.08);
        -webkit-backdrop-filter: blur(28px) saturate(1.08);
        background: rgba(15, 23, 42, 0.34);
        border: 1px solid rgba(148, 163, 184, 0.14);
        box-shadow:
          inset 0 1px 0 rgba(255, 255, 255, 0.08),
          0 24px 60px rgba(2, 6, 23, 0.28);
      }
      .startup-shell__sidebar {
        border-radius: 28px;
        padding: 22px 18px;
        display: flex;
        flex-direction: column;
        gap: 24px;
      }
      .startup-shell__brand {
        display: flex;
        align-items: center;
        gap: 14px;
      }
      .startup-shell__brand-bar {
        display: block;
        width: 124px;
        height: 14px;
        border-radius: 999px;
      }
      .startup-shell__brand-mark {
        width: 42px;
        height: 42px;
        border-radius: 14px;
        background:
          linear-gradient(135deg, rgba(96, 165, 250, 0.96), rgba(14, 165, 233, 0.58)),
          rgba(15, 23, 42, 0.72);
        box-shadow:
          inset 0 1px 0 rgba(255, 255, 255, 0.24),
          0 12px 32px rgba(14, 165, 233, 0.3);
      }
      .startup-shell__sidebar-card {
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 18px;
        border-radius: 22px;
        background: rgba(15, 23, 42, 0.26);
      }
      .startup-shell__sidebar-title {
        display: block;
        width: 52%;
        height: 12px;
        border-radius: 999px;
      }
      .startup-shell__sidebar-row {
        display: block;
        width: var(--row-width, 100%);
        height: 12px;
        border-radius: 999px;
      }
      .startup-shell__main {
        display: flex;
        flex-direction: column;
        gap: 18px;
      }
      .startup-shell__topbar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
      }
      .startup-shell__chip,
      .startup-shell__status-wrap {
        display: inline-flex;
        align-items: center;
        gap: 10px;
        padding: 10px 14px;
        border-radius: 999px;
        background: rgba(15, 23, 42, 0.28);
        border: 1px solid rgba(148, 163, 184, 0.14);
        backdrop-filter: blur(18px);
        -webkit-backdrop-filter: blur(18px);
      }
      .startup-shell__chip {
        width: 90px;
        height: 14px;
        padding: 0;
        border-radius: 999px;
      }
      .startup-shell__status-wrap--icon-only {
        padding-inline: 12px;
      }
      .startup-shell__spinner {
        width: 16px;
        height: 16px;
        border-radius: 999px;
        border: 2px solid rgba(148, 163, 184, 0.42);
        border-top-color: rgba(96, 165, 250, 0.98);
        animation: startup-shell-spin .9s linear infinite;
      }
      .startup-shell__hero {
        padding: 8px 6px 0;
        max-width: 720px;
        display: flex;
        flex-direction: column;
        gap: 12px;
      }
      .startup-shell__hero-kicker,
      .startup-shell__hero-title,
      .startup-shell__hero-copy {
        display: block;
        border-radius: 999px;
      }
      .startup-shell__hero-kicker {
        width: 116px;
        height: 12px;
      }
      .startup-shell__hero-title {
        width: min(100%, 460px);
        height: clamp(28px, 4vw, 40px);
      }
      .startup-shell__hero-copy {
        width: min(100%, 520px);
        height: 12px;
      }
      .startup-shell__hero-copy--short {
        width: min(100%, 360px);
      }
      .startup-shell__surface {
        border-radius: 30px;
        padding: 24px;
        display: flex;
        flex-direction: column;
        gap: 18px;
        min-height: 420px;
      }
      .startup-shell__message {
        display: flex;
        gap: 12px;
        align-items: flex-start;
      }
      .startup-shell__message--user {
        justify-content: flex-end;
      }
      .startup-shell__avatar {
        width: 32px;
        height: 32px;
        border-radius: 11px;
        background: linear-gradient(135deg, rgba(96, 165, 250, 0.95), rgba(14, 165, 233, 0.48));
        box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18);
        flex: 0 0 auto;
      }
      .startup-shell__message-body {
        min-width: min(100%, 560px);
        max-width: min(100%, 620px);
        padding: 18px 18px 16px;
        border-radius: 24px 24px 24px 10px;
        background: rgba(15, 23, 42, 0.28);
        border: 1px solid rgba(148, 163, 184, 0.12);
        display: flex;
        flex-direction: column;
        gap: 10px;
      }
      .startup-shell__message-body--user {
        max-width: min(100%, 300px);
        border-radius: 24px 24px 10px 24px;
        background: rgba(59, 130, 246, 0.18);
      }
      .startup-shell__line {
        display: block;
        width: var(--line-width, 100%);
        height: 12px;
        border-radius: 999px;
      }
      .startup-shell__composer {
        margin-top: auto;
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding-top: 12px;
      }
      .startup-shell__composer-bar {
        display: block;
        width: 100%;
        height: 54px;
        border-radius: 18px;
      }
      .startup-shell__composer-actions {
        display: flex;
        align-items: center;
        gap: 10px;
      }
      .startup-shell__composer-pill {
        display: block;
        width: 92px;
        height: 12px;
        border-radius: 999px;
      }
      .startup-shell__composer-send {
        margin-left: auto;
        width: 42px;
        height: 42px;
        border-radius: 14px;
        background: linear-gradient(135deg, rgba(59, 130, 246, 0.88), rgba(14, 165, 233, 0.62));
        box-shadow:
          inset 0 1px 0 rgba(255, 255, 255, 0.18),
          0 12px 24px rgba(14, 165, 233, 0.24);
      }
      .startup-shell__composer-copy {
        display: block;
        width: 240px;
        height: 12px;
        border-radius: 999px;
      }
      .startup-shell__skeleton {
        position: relative;
        overflow: hidden;
        background: rgba(148, 163, 184, 0.18);
      }
      .startup-shell__skeleton::after {
        content: '';
        position: absolute;
        inset: 0;
        transform: translateX(-100%);
        background: linear-gradient(
          90deg,
          transparent,
          rgba(255, 255, 255, 0.18),
          transparent
        );
        animation: startup-shell-shimmer 1.45s ease-in-out infinite;
      }
      @keyframes startup-shell-spin {
        to {
          transform: rotate(360deg);
        }
      }
      @keyframes startup-shell-shimmer {
        100% {
          transform: translateX(100%);
        }
      }
      @media (max-width: 900px) {
        .startup-shell__layout {
          grid-template-columns: minmax(0, 1fr);
          padding: 22px 18px;
        }
        .startup-shell__sidebar {
          display: none;
        }
        .startup-shell__surface {
          padding: 20px;
          min-height: 360px;
        }
      }
      @media (max-width: 640px) {
        .startup-shell__topbar {
          flex-direction: column;
          align-items: flex-start;
        }
        .startup-shell__hero h1 {
          font-size: 30px;
        }
        .startup-shell__message-body {
          min-width: 0;
        }
      }
      @media (prefers-color-scheme: light) {
        .startup-shell {
          color: rgba(15, 23, 42, 0.94);
          background:
            radial-gradient(circle at top left, rgba(59, 130, 246, 0.12), transparent 34%),
            radial-gradient(circle at top right, rgba(34, 197, 94, 0.08), transparent 30%),
            linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(241, 245, 249, 0.98));
        }
        .startup-shell__sidebar,
        .startup-shell__surface,
        .startup-shell__chip,
        .startup-shell__status-wrap {
          background: rgba(255, 255, 255, 0.58);
          border-color: rgba(148, 163, 184, 0.18);
          box-shadow:
            inset 0 1px 0 rgba(255, 255, 255, 0.78),
            0 24px 60px rgba(148, 163, 184, 0.2);
        }
        .startup-shell__message-body {
          background: rgba(255, 255, 255, 0.62);
          border-color: rgba(148, 163, 184, 0.18);
        }
        .startup-shell__message-body--user {
          background: rgba(191, 219, 254, 0.52);
        }
        .startup-shell__skeleton {
          background: rgba(148, 163, 184, 0.22);
        }
      }
    </style>
  `;
})();
"##;

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
struct TrayPanelItem(MenuItem<tauri::Wry>);
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
        .visible(false)
        .initialization_script(ABOUT_BLANK_SPLASH_SCRIPT);

    #[cfg(target_os = "macos")]
    let builder = builder
        .transparent(true)
        .title_bar_style(tauri::TitleBarStyle::Overlay)
        .traffic_light_position(tauri::LogicalPosition::new(18.0, 18.0))
        .hidden_title(true)
        .accept_first_mouse(true)
        .effects(main_window_effects())
        .initialization_script(MACOS_GLASS_INIT_SCRIPT);

    builder.build()
}

fn build_panel_window<R: tauri::Runtime, M: Manager<R>>(
    manager: &M,
    url: tauri::WebviewUrl,
) -> tauri::Result<tauri::WebviewWindow<R>> {
    let builder = tauri::WebviewWindowBuilder::new(manager, PANEL_WINDOW_LABEL, url)
        .title(PANEL_WINDOW_TITLE)
        .inner_size(PANEL_WINDOW_WIDTH, PANEL_WINDOW_HEIGHT)
        .min_inner_size(PANEL_WINDOW_MIN_WIDTH, PANEL_WINDOW_MIN_HEIGHT)
        .center()
        .resizable(true)
        .maximizable(false)
        .minimizable(false)
        .skip_taskbar(true)
        .visible(false)
        .initialization_script(ABOUT_BLANK_SPLASH_SCRIPT);

    #[cfg(target_os = "macos")]
    let builder = builder
        .transparent(true)
        .title_bar_style(tauri::TitleBarStyle::Overlay)
        .traffic_light_position(tauri::LogicalPosition::new(18.0, 18.0))
        .hidden_title(true)
        .accept_first_mouse(true)
        .effects(main_window_effects())
        .initialization_script(MACOS_GLASS_INIT_SCRIPT);

    builder.build()
}

fn about_blank_webview_url() -> tauri::WebviewUrl {
    tauri::WebviewUrl::CustomProtocol(
        "about:blank"
            .parse()
            .expect("about:blank should be a valid URL"),
    )
}

fn server_origin_from_parts(server_running: bool, port: u16, use_https: bool) -> Option<String> {
    if !server_running {
        return None;
    }

    let protocol = if use_https { "https" } else { "http" };
    Some(format!("{}://localhost:{}", protocol, port))
}

fn server_origin(app_handle: &tauri::AppHandle) -> Option<String> {
    let state = app_handle.try_state::<AppState>()?;
    let server_running = *state.server_running.lock().unwrap();
    let port = *state.server_port.lock().unwrap();
    let use_https = *state.use_https.lock().unwrap();
    server_origin_from_parts(server_running, port, use_https)
}

fn webview_url_for_path(app_handle: &tauri::AppHandle, path: &str) -> tauri::WebviewUrl {
    if let Some(origin) = server_origin(app_handle) {
        tauri::WebviewUrl::External(
            format!("{}{}", origin, path)
                .parse()
                .expect("server URL should be valid"),
        )
    } else {
        about_blank_webview_url()
    }
}

fn normalize_server_restart_path(path: Option<&str>) -> String {
    let Some(raw_path) = path else {
        return "/".to_string();
    };

    let trimmed = raw_path.trim();
    if trimmed.is_empty() {
        return "/".to_string();
    }

    if trimmed.starts_with('/') {
        return trimmed.to_string();
    }

    if trimmed.starts_with('?') || trimmed.starts_with('#') {
        return format!("/{}", trimmed);
    }

    format!("/{}", trimmed)
}

fn show_and_focus_window<R: tauri::Runtime>(window: &tauri::WebviewWindow<R>) {
    let _ = window.show();
    let _ = window.unminimize();
    let _ = window.set_focus();
}

#[cfg(target_os = "macos")]
fn macos_app_bundle_path(exe: &Path) -> Option<PathBuf> {
    let macos_dir = exe.parent()?;
    if macos_dir.file_name()? != "MacOS" {
        return None;
    }

    let contents_dir = macos_dir.parent()?;
    if contents_dir.file_name()? != "Contents" {
        return None;
    }

    let app_dir = contents_dir.parent()?;
    if app_dir.extension()? != "app" {
        return None;
    }

    Some(app_dir.to_path_buf())
}

#[cfg(target_os = "macos")]
fn macos_parent_pid() -> u32 {
    let parent = unsafe { getppid() };
    if parent <= 0 {
        0
    } else {
        parent as u32
    }
}

#[cfg(target_os = "macos")]
fn should_relaunch_bundle_via_open(exe: &Path, parent_pid: u32) -> bool {
    parent_pid != 1 && macos_app_bundle_path(exe).is_some()
}

#[cfg(target_os = "macos")]
fn maybe_relaunch_bundle_via_open() -> bool {
    let exe = match std::env::current_exe() {
        Ok(exe) => exe,
        Err(err) => {
            eprintln!(
                "Unable to determine current executable for macOS relaunch: {}",
                err
            );
            return false;
        }
    };

    let parent_pid = macos_parent_pid();
    if !should_relaunch_bundle_via_open(&exe, parent_pid) {
        return false;
    }

    let Some(app_bundle) = macos_app_bundle_path(&exe) else {
        return false;
    };

    let mut command = std::process::Command::new("open");
    command.arg("-n").arg(&app_bundle);

    let args: Vec<_> = std::env::args_os().skip(1).collect();
    if !args.is_empty() {
        command.arg("--args");
        command.args(args);
    }

    match command.status() {
        Ok(status) if status.success() => {
            eprintln!(
                "Relaunching {} via LaunchServices to avoid direct-binary AppKit startup crashes.",
                app_bundle.display()
            );
            true
        }
        Ok(status) => {
            eprintln!(
                "Failed to relaunch {} via LaunchServices (exit status: {}).",
                app_bundle.display(),
                status
            );
            false
        }
        Err(err) => {
            eprintln!(
                "Failed to relaunch {} via LaunchServices: {}",
                app_bundle.display(),
                err
            );
            false
        }
    }
}

#[cfg(target_os = "macos")]
fn update_window_on_main_thread(
    app_handle: &tauri::AppHandle,
    label: &str,
    apply_style: bool,
    focus: bool,
) {
    let app_handle = app_handle.clone();
    let callback_handle = app_handle.clone();
    let label = label.to_string();
    let _ = app_handle.run_on_main_thread(move || {
        let trace_main_window = label == MAIN_WINDOW_LABEL;
        let Some(window) = callback_handle.get_webview_window(&label) else {
            return;
        };

        if trace_main_window {
            DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_enter");
        }

        if focus {
            if trace_main_window {
                DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_activate_begin");
            }
            activate_macos_app();
            if trace_main_window {
                DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_activate_complete");
            }
        }

        if apply_style {
            if trace_main_window {
                DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_style_begin");
            }
            apply_main_window_macos_style(&window);
            if trace_main_window {
                DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_style_complete");
            }
        }

        if trace_main_window {
            DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_show_begin");
        }
        let _ = window.show();
        let _ = window.unminimize();
        if focus {
            if let Ok(ns_window_ptr) = window.ns_window() {
                let ns_window: &objc2_app_kit::NSWindow = unsafe { &*ns_window_ptr.cast() };
                ns_window.orderFrontRegardless();
            }
            let _ = window.set_focus();
        }
        if trace_main_window {
            DESKTOP_STARTUP_TRACE.mark("main_window_main_thread_show_complete");
        }
    });
}

fn rebuild_window_for_path(
    app_handle: &tauri::AppHandle,
    label: &str,
    path: &str,
) -> Result<tauri::WebviewWindow, String> {
    if let Some(window) = app_handle.get_webview_window(label) {
        let _ = window.destroy();
    }

    match label {
        MAIN_WINDOW_LABEL => build_main_window(app_handle, webview_url_for_path(app_handle, path))
            .map_err(|e| e.to_string()),
        PANEL_WINDOW_LABEL => {
            build_panel_window(app_handle, webview_url_for_path(app_handle, path))
                .map_err(|e| e.to_string())
        }
        _ => Err(format!("Unknown window label {label}")),
    }
}

#[cfg(target_os = "macos")]
fn apply_main_window_macos_style<R: tauri::Runtime>(window: &tauri::WebviewWindow<R>) {
    use objc2_app_kit::{NSWindow, NSWindowToolbarStyle};

    if let Err(err) = window.set_title_bar_style(tauri::TitleBarStyle::Overlay) {
        error!("Failed to apply macOS title bar style: {}", err);
    }
    if let Err(err) = window.set_effects(main_window_effects()) {
        error!("Failed to apply macOS window effects: {}", err);
    }

    let ns_window_ptr = match window.ns_window() {
        Ok(ptr) => ptr,
        Err(err) => {
            error!("Failed to access macOS NSWindow handle: {}", err);
            return;
        }
    };

    let ns_window: &NSWindow = unsafe { &*ns_window_ptr.cast() };
    ns_window.setTitlebarAppearsTransparent(true);
    ns_window.setToolbarStyle(NSWindowToolbarStyle::UnifiedCompact);
}

#[cfg(target_os = "macos")]
fn activate_macos_app() {
    use objc2::MainThreadMarker;
    use objc2_app_kit::{NSApplication, NSApplicationActivationPolicy};

    if let Some(mtm) = MainThreadMarker::new() {
        let ns_app = NSApplication::sharedApplication(mtm);
        ns_app.setActivationPolicy(NSApplicationActivationPolicy::Regular);
        ns_app.unhide(None);
        ns_app.activate();
        #[allow(deprecated)]
        ns_app.activateIgnoringOtherApps(true);
    }
}

fn open_or_focus_main_window(app_handle: &tauri::AppHandle) -> tauri::Result<()> {
    if let Some(_window) = app_handle.get_webview_window(MAIN_WINDOW_LABEL) {
        #[cfg(target_os = "macos")]
        update_window_on_main_thread(app_handle, MAIN_WINDOW_LABEL, true, true);
        #[cfg(not(target_os = "macos"))]
        show_and_focus_window(&_window);
        return Ok(());
    }

    let _window = build_main_window(app_handle, webview_url_for_path(app_handle, ""))?;
    #[cfg(target_os = "macos")]
    update_window_on_main_thread(app_handle, MAIN_WINDOW_LABEL, true, true);
    #[cfg(not(target_os = "macos"))]
    show_and_focus_window(&_window);
    Ok(())
}

fn open_or_focus_panel_window(app_handle: &tauri::AppHandle) -> tauri::Result<()> {
    if let Some(_window) = app_handle.get_webview_window(PANEL_WINDOW_LABEL) {
        #[cfg(target_os = "macos")]
        update_window_on_main_thread(app_handle, PANEL_WINDOW_LABEL, true, true);
        #[cfg(not(target_os = "macos"))]
        show_and_focus_window(&_window);
        return Ok(());
    }

    let _window = build_panel_window(
        app_handle,
        webview_url_for_path(app_handle, PANEL_WINDOW_PATH),
    )?;
    #[cfg(target_os = "macos")]
    update_window_on_main_thread(app_handle, PANEL_WINDOW_LABEL, true, true);
    #[cfg(not(target_os = "macos"))]
    show_and_focus_window(&_window);
    Ok(())
}

fn navigate_window_to_server_path(
    app_handle: &tauri::AppHandle,
    label: &str,
    path: &str,
) -> Result<(), String> {
    if server_origin(app_handle).is_none() {
        return Err("Server URL not ready".to_string());
    }

    let Some(window) = app_handle.get_webview_window(label) else {
        return Err(format!("Window {label} not found"));
    };

    let url = match webview_url_for_path(app_handle, path) {
        tauri::WebviewUrl::External(url) => url,
        _ => return Err(format!("Server URL for window {label} is not ready")),
    };

    window.navigate(url).map_err(|e| e.to_string())
}

fn bind_window_to_server_path(
    app_handle: &tauri::AppHandle,
    label: &str,
    path: &str,
) -> Result<tauri::WebviewWindow, String> {
    let traced_path = startup_trace_path(path);

    if app_handle.get_webview_window(label).is_none() {
        info!("Window {label} missing after server startup; creating it");
        return rebuild_window_for_path(app_handle, label, &traced_path);
    }

    info!("Navigating window {label} to the server-backed UI");
    navigate_window_to_server_path(app_handle, label, &traced_path)?;

    app_handle
        .get_webview_window(label)
        .ok_or_else(|| format!("Window {label} disappeared after navigation"))
}

fn show_server_start_error<R: tauri::Runtime>(
    window: &tauri::WebviewWindow<R>,
    error_message: &str,
) {
    let escaped = error_message
        .replace('\\', "\\\\")
        .replace('\'', "\\'")
        .replace('\n', "\\n");
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
    show_and_focus_window(window);
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

fn quick_panel_label_for_locale(locale: &str) -> &'static str {
    let prefix = locale.split(&['-', '_'][..]).next().unwrap_or("en");
    match prefix {
        "zh" => "打开快捷面板",
        "ja" => "クイックパネルを開く",
        "ko" => "빠른 패널 열기",
        "fr" => "Ouvrir le panneau rapide",
        "de" => "Schnellpanel öffnen",
        "es" => "Abrir panel rapido",
        "it" => "Apri pannello rapido",
        "pt" => "Abrir painel rapido",
        "nl" => "Snelpaneel openen",
        "ca" => "Obre el panell rapid",
        "sv" => "Oppna snabbpanel",
        "da" => "Abn hurtigpanel",
        "nb" | "no" => "Apne hurtigpanel",
        "ga" => "Oscail an painel tapa",
        "pl" => "Otworz szybki panel",
        "cs" => "Otevrit rychly panel",
        "sk" => "Otvorit rychly panel",
        "hu" => "Gyorspanel megnyitasa",
        "ro" => "Deschide panoul rapid",
        "hr" => "Otvori brzu plocu",
        "el" => "Ανοιγμα γρηγορου πινακα",
        "ru" => "Открыть быструю панель",
        "ml" => "ദ്രുത പാനല് തുറക്കുക",
        _ => "Open Quick Panel",
    }
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

fn startup_trace_env_enabled() -> bool {
    ["ZIMAOS_STARTUP_TRACE", "BLUE_STARTUP_TRACE"]
        .iter()
        .filter_map(|key| std::env::var(key).ok())
        .any(|raw| parse_bool_env_flag(&raw).unwrap_or(false))
}

fn startup_trace_disable_notification_plugin() -> bool {
    if !startup_trace_env_enabled() {
        return false;
    }

    [
        "ZIMAOS_STARTUP_TRACE_DISABLE_NOTIFICATION_PLUGIN",
        "BLUE_STARTUP_TRACE_DISABLE_NOTIFICATION_PLUGIN",
    ]
    .iter()
    .filter_map(|key| std::env::var(key).ok())
    .any(|raw| parse_bool_env_flag(&raw).unwrap_or(false))
}

fn startup_trace_path(path: &str) -> String {
    if !startup_trace_env_enabled() || path.contains("startup_trace=") {
        return path.to_string();
    }

    let separator = if path.contains('?') { '&' } else { '?' };
    format!("{path}{separator}startup_trace=1")
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

fn cli_compatible_data_dir_for_home(home_dir: &Path, dev_mode: bool) -> PathBuf {
    let state_dir = if dev_mode {
        ".zimaos-blue-dev"
    } else {
        ".zimaos-blue"
    };
    home_dir.join(state_dir).join("data")
}

#[cfg(target_os = "macos")]
fn default_macos_data_dir(cli_dev_mode: bool) -> Option<String> {
    dirs::home_dir().map(|home| {
        cli_compatible_data_dir_for_home(&home, cli_dev_mode)
            .to_string_lossy()
            .to_string()
    })
}

#[cfg(any(target_os = "macos", target_os = "windows"))]
fn embedded_server_data_dir(cli: &CliArgs) -> String {
    if let Some(path) = cli.data_dir.clone() {
        return path;
    }

    #[cfg(target_os = "macos")]
    if let Some(path) = default_macos_data_dir(cli.dev) {
        return path;
    }

    std::env::current_exe()
        .ok()
        .and_then(|exe| exe.parent().map(|p| p.join("data")))
        .and_then(|p| p.to_str().map(|s| s.to_string()))
        .unwrap_or_else(|| {
            dirs::home_dir()
                .map(|h| h.join(".zimaos-blue").to_string_lossy().to_string())
                .unwrap_or_else(|| ".zimaos-blue".to_string())
        })
}

#[cfg(test)]
mod tests {
    use super::{
        build_args_string, cli_compatible_data_dir_for_home, embedded_server_port_bind_timeout,
        normalize_server_restart_path, parent_directory_for_reveal_fallback, parse_bool_env_flag,
        reveal_path_with_fallback, server_origin_from_parts, stt_auth_startup_enabled,
        ABOUT_BLANK_SPLASH_SCRIPT, CliArgs,
    };
    #[cfg(target_os = "macos")]
    use super::{macos_app_bundle_path, should_relaunch_bundle_via_open};
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
    fn embedded_server_port_bind_timeout_is_more_lenient_for_debug_builds() {
        assert_eq!(
            embedded_server_port_bind_timeout(false),
            std::time::Duration::from_secs(45)
        );
        assert_eq!(
            embedded_server_port_bind_timeout(true),
            std::time::Duration::from_secs(90)
        );
        assert!(embedded_server_port_bind_timeout(true) > embedded_server_port_bind_timeout(false));
    }

    #[test]
    fn about_blank_splash_script_renders_startup_shell_skeleton() {
        assert!(ABOUT_BLANK_SPLASH_SCRIPT.contains("startup-shell"));
        assert!(ABOUT_BLANK_SPLASH_SCRIPT.contains("startup-shell__surface"));
        assert!(ABOUT_BLANK_SPLASH_SCRIPT.contains("startup-shell__hero-title"));
        assert!(ABOUT_BLANK_SPLASH_SCRIPT.contains("startup-shell__composer-bar"));
    }

    #[test]
    fn server_origin_from_parts_returns_none_when_server_not_running() {
        assert_eq!(server_origin_from_parts(false, 80, false), None);
    }

    #[test]
    fn server_origin_from_parts_uses_actual_fallback_port_when_running() {
        assert_eq!(
            server_origin_from_parts(true, 43127, false),
            Some("http://localhost:43127".to_string())
        );
        assert_eq!(
            server_origin_from_parts(true, 43127, true),
            Some("https://localhost:43127".to_string())
        );
    }

    #[test]
    fn normalize_server_restart_path_preserves_internal_routes() {
        assert_eq!(
            normalize_server_restart_path(Some("/settings?tab=userdata#backup")),
            "/settings?tab=userdata#backup".to_string()
        );
        assert_eq!(
            normalize_server_restart_path(Some("?tab=userdata")),
            "/?tab=userdata".to_string()
        );
        assert_eq!(
            normalize_server_restart_path(Some("settings")),
            "/settings".to_string()
        );
        assert_eq!(normalize_server_restart_path(Some("   ")), "/".to_string());
        assert_eq!(normalize_server_restart_path(None), "/".to_string());
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
    fn cli_compatible_data_dir_for_home_matches_cli_layout() {
        let home = Path::new("/Users/tester");
        assert_eq!(
            cli_compatible_data_dir_for_home(home, false),
            PathBuf::from("/Users/tester/.zimaos-blue/data")
        );
        assert_eq!(
            cli_compatible_data_dir_for_home(home, true),
            PathBuf::from("/Users/tester/.zimaos-blue-dev/data")
        );
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn build_args_string_injects_cli_compatible_default_data_dir_on_macos() {
        let cli = CliArgs::default();
        let args = build_args_string(&cli, None).expect("expected default data-dir args");
        let expected = format!(
            "--data-dir {}",
            cli_compatible_data_dir_for_home(
                &dirs::home_dir().expect("home directory should be available"),
                false
            )
            .to_string_lossy()
        );
        assert_eq!(args, expected);
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn build_args_string_uses_dev_cli_compatible_default_data_dir_on_macos() {
        let cli = CliArgs {
            dev: true,
            ..CliArgs::default()
        };
        let args = build_args_string(&cli, None).expect("expected default data-dir args");
        let expected = format!(
            "--data-dir {} --dev",
            cli_compatible_data_dir_for_home(
                &dirs::home_dir().expect("home directory should be available"),
                true
            )
            .to_string_lossy()
        );
        assert_eq!(args, expected);
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn build_args_string_preserves_explicit_data_dir_on_macos() {
        let cli = CliArgs {
            data_dir: Some("/tmp/custom-data".to_string()),
            dev: true,
            ..CliArgs::default()
        };
        let args = build_args_string(&cli, None).expect("expected explicit data-dir args");
        assert_eq!(args, "--data-dir /tmp/custom-data --dev");
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn macos_app_bundle_path_detects_bundle_binary() {
        let exe = PathBuf::from("/Applications/ZimaOS Blue.app/Contents/MacOS/blue");
        assert_eq!(
            macos_app_bundle_path(&exe),
            Some(PathBuf::from("/Applications/ZimaOS Blue.app"))
        );
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn macos_app_bundle_path_ignores_non_bundle_binary() {
        assert_eq!(
            macos_app_bundle_path(Path::new("/usr/local/bin/blue")),
            None
        );
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn should_relaunch_bundle_via_open_only_for_direct_bundle_launches() {
        let exe = PathBuf::from("/Applications/ZimaOS Blue.app/Contents/MacOS/blue");
        assert!(should_relaunch_bundle_via_open(&exe, 3538));
        assert!(!should_relaunch_bundle_via_open(&exe, 1));
        assert!(!should_relaunch_bundle_via_open(
            Path::new("/usr/local/bin/blue"),
            3538
        ));
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
    let quit_label = quit_label_for_locale(&locale);
    let panel_label = quick_panel_label_for_locale(&locale);
    if let Some(panel_item) = app.try_state::<TrayPanelItem>() {
        let _ = panel_item.0.set_text(panel_label);
    }
    if let Some(quit_item) = app.try_state::<TrayQuitItem>() {
        let _ = quit_item.0.set_text(&quit_label);
    }
    info!(
        "Tray menu language updated to: {} (panel: {}, quit: {})",
        locale, panel_label, quit_label
    );
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
    DESKTOP_STARTUP_TRACE.mark("server_platform_enter");

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

        let data_dir = if let Some(state) = app.try_state::<AppState>() {
            embedded_server_data_dir(&state.cli_args)
        } else {
            #[cfg(target_os = "macos")]
            {
                default_macos_data_dir(false).unwrap_or_else(|| ".zimaos-blue/data".to_string())
            }
            #[cfg(target_os = "windows")]
            {
                std::env::current_exe()
                    .ok()
                    .and_then(|exe| exe.parent().map(|p| p.join("data")))
                    .and_then(|p| p.to_str().map(|s| s.to_string()))
                    .unwrap_or_else(|| ".\\data".to_string())
            }
        };

        DESKTOP_STARTUP_TRACE.mark("ffi_start_requested");
        blue_ffi::start_server_with_args(port, Some(&data_dir), cli_args_str.as_deref())?;
        DESKTOP_STARTUP_TRACE.mark("ffi_start_returned");

        if let Some(state) = app.try_state::<AppState>() {
            *state.server_running.lock().unwrap() = true;
        }

        // Wait for Go server to be ready using two-phase detection:
        // Phase 1: Poll BlueServerIsRunning() + BlueServerGetPort() via FFI.
        //          Wait until the Go goroutine has started AND bound a port.
        // Phase 2: HTTP health check to confirm the listener is accepting connections.
        let mut use_https = false;
        let mut actual_port: u16 = port;

        // Phase 1: FFI poll — wait for is_running + port > 0.
        // Embedded startup can take a few seconds on first launch or slower machines,
        // so avoid treating a healthy but slower boot as a fatal error.
        let mut phase1_ok = false;
        let phase1_timeout = embedded_server_port_bind_timeout(cfg!(debug_assertions));
        let phase1_deadline = std::time::Instant::now() + phase1_timeout;
        while std::time::Instant::now() < phase1_deadline {
            if blue_ffi::is_running() {
                let p = blue_ffi::get_port();
                if p > 0 {
                    actual_port = p;
                    phase1_ok = true;
                    break;
                }
            }
            tokio::time::sleep(std::time::Duration::from_millis(25)).await;
        }

        if !phase1_ok {
            return Err(format!(
                "Server failed to start: timed out after {:?} waiting for embedded server port binding",
                phase1_timeout
            ));
        }
        DESKTOP_STARTUP_TRACE.mark("ffi_port_bound");

        info!("Server bound to port {}", actual_port);

        // Update state with actual port
        if let Some(state) = app.try_state::<AppState>() {
            *state.server_port.lock().unwrap() = actual_port;
        }

        // Phase 2: HTTP health check — server goroutine is running, listener may be up
        let http_url = format!("http://localhost:{}/api/v1/health", actual_port);
        let client = reqwest::Client::builder()
            .danger_accept_invalid_certs(true)
            .timeout(std::time::Duration::from_secs(1))
            .build()
            .unwrap_or_else(|_| reqwest::Client::new());

        let mut server_ready = false;
        for i in 0..40 {
            if i > 0 {
                // Local loopback health checks are cheap; keep the early polling cadence
                // tight so we don't oversleep after the listener is already ready.
                let delay_ms = if i < 10 {
                    15
                } else if i < 25 {
                    30
                } else {
                    60
                };
                tokio::time::sleep(std::time::Duration::from_millis(delay_ms)).await;
            }

            if client.get(&http_url).send().await.is_ok() {
                info!("Server ready (HTTP) after attempt {}", i + 1);
                server_ready = true;
                break;
            }

            // Only try HTTPS after a few HTTP failures (rare case: TLS enabled)
            if i >= 8 {
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
        DESKTOP_STARTUP_TRACE.mark("http_health_ready");

        // Update state with detected protocol
        if let Some(state) = app.try_state::<AppState>() {
            *state.use_https.lock().unwrap() = use_https;
        }
        DESKTOP_STARTUP_TRACE.mark("protocol_detected");

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
    } else {
        #[cfg(target_os = "macos")]
        if let Some(d) = default_macos_data_dir(cli.dev) {
            parts.push(format!("--data-dir {}", d));
        }
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

fn embedded_server_port_bind_timeout(is_debug_build: bool) -> std::time::Duration {
    if is_debug_build {
        std::time::Duration::from_secs(90)
    } else {
        std::time::Duration::from_secs(45)
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

#[tauri::command]
async fn restart_server_runtime(
    app: tauri::AppHandle,
    path: Option<String>,
) -> Result<String, String> {
    let target_path = normalize_server_restart_path(path.as_deref());
    info!(
        "Restarting desktop-managed server runtime and rebinding windows to {}",
        target_path
    );

    stop_server_platform(&app).await?;
    tokio::time::sleep(std::time::Duration::from_millis(200)).await;
    start_server_platform(&app).await?;

    let _ = bind_window_to_server_path(&app, MAIN_WINDOW_LABEL, &target_path)?;
    if app.get_webview_window(PANEL_WINDOW_LABEL).is_some() {
        if let Err(err) = bind_window_to_server_path(&app, PANEL_WINDOW_LABEL, PANEL_WINDOW_PATH) {
            warn!("Failed to rebind quick panel after server restart: {}", err);
        }
    }

    server_origin(&app).ok_or_else(|| "Server URL not ready after runtime restart".to_string())
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

    #[cfg(target_os = "macos")]
    if maybe_relaunch_bundle_via_open() {
        return;
    }

    #[cfg(not(debug_assertions))]
    let startup_trace_enabled = startup_trace_env_enabled();

    // Initialize logger with optimized settings for Windows
    // Use warn level in release builds to reduce startup overhead
    #[cfg(debug_assertions)]
    let default_level = if cli_args.verbose { "debug" } else { "info" };
    #[cfg(not(debug_assertions))]
    let default_level = if cli_args.verbose {
        "debug"
    } else if startup_trace_enabled {
        "info"
    } else {
        "warn"
    };

    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or(default_level))
        .format_timestamp(None)
        .format_module_path(false)
        .format_target(false)
        .format_level(false)
        .init();

    info!("Starting ZimaOS Blue desktop application");
    DESKTOP_STARTUP_TRACE.mark("run_enter");
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
    DESKTOP_STARTUP_TRACE.mark("app_state_ready");

    let startup_trace_enabled = startup_trace_env_enabled();
    let disable_notification_plugin = startup_trace_disable_notification_plugin();

    let builder = tauri::Builder::default();
    DESKTOP_STARTUP_TRACE.mark("builder_default_ready");

    let builder = if startup_trace_enabled {
        info!("Startup trace enabled; single-instance plugin disabled for clean profiling");
        builder
    } else {
        builder.plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // When second instance is launched, show and focus the first instance
            info!("Second instance detected, focusing existing window");
            if let Err(e) = open_or_focus_main_window(&app) {
                error!("Failed to focus main window for second instance: {}", e);
            }
        }))
    };
    DESKTOP_STARTUP_TRACE.mark("single_instance_configured");

    let builder = builder.plugin(tauri_plugin_shell::init());
    DESKTOP_STARTUP_TRACE.mark("plugin_shell_registered");

    let builder = if disable_notification_plugin {
        info!("Startup trace profiling: notification plugin disabled");
        builder
    } else {
        let builder = builder.plugin(tauri_plugin_notification::init());
        DESKTOP_STARTUP_TRACE.mark("plugin_notification_registered");
        builder
    };

    let builder = builder.plugin(tauri_plugin_process::init());
    DESKTOP_STARTUP_TRACE.mark("plugin_process_registered");

    let builder = builder.plugin(tauri_plugin_os::init());
    DESKTOP_STARTUP_TRACE.mark("plugin_os_registered");

    let builder = builder.plugin(tauri_plugin_opener::init());
    DESKTOP_STARTUP_TRACE.mark("plugin_opener_registered");

    let builder = builder
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
            restart_server_runtime,
            server::get_server_status,
        ])
        .on_page_load(|webview, payload| {
            let url = payload.url().to_string();

            // For about:blank, inject a splash spinner and show the window immediately.
            // This gives instant visual feedback while the Go server boots.
            if url == "about:blank" {
                DESKTOP_STARTUP_TRACE.mark("about_blank_page_loaded");
                let _ = webview.eval(ABOUT_BLANK_SPLASH_SCRIPT);
                let _ = webview.window().show();
                let _ = webview.window().set_focus();
            }

            // Show the window when the localhost page loads.
            if url.contains("localhost") {
                DESKTOP_STARTUP_TRACE.mark("localhost_page_loaded");
                let _ = webview.window().show();
                let _ = webview.window().set_focus();
            }

            // Inject desktop markers into external localhost pages so the frontend knows
            // it's running inside the desktop app. Tauri IPC for these pages is controlled
            // by the configured remote capabilities in tauri.conf.json.
            if url.contains("localhost") {
                #[cfg(target_os = "macos")]
                let _ = webview.eval(
                    "window.__BLUE_DESKTOP__=true;\
                     window.__BLUE_MACOS_GLASS__=true;\
                     document.documentElement.dataset.blueDesktop='true';\
                     document.documentElement.dataset.blueMacosGlass='true';",
                );

                #[cfg(not(target_os = "macos"))]
                let _ = webview.eval(
                    "window.__BLUE_DESKTOP__=true;\
                     document.documentElement.dataset.blueDesktop='true';",
                );
            }
        })
        .setup(|app| {
            info!("Setting up application");
            DESKTOP_STARTUP_TRACE.mark("setup_enter");

            if app.get_webview_window(MAIN_WINDOW_LABEL).is_none() {
                #[cfg(target_os = "macos")]
                info!("Creating main window with macOS vibrancy styling");

                #[cfg(not(target_os = "macos"))]
                info!("Creating main window");

                let window = build_main_window(app.handle(), about_blank_webview_url())?;
                show_and_focus_window(&window);
                DESKTOP_STARTUP_TRACE.mark("main_window_visible");
            } else {
                #[cfg(target_os = "macos")]
                if let Some(window) = app.get_webview_window(MAIN_WINDOW_LABEL) {
                    apply_main_window_macos_style(&window);
                }
            }

            // Detect system language for initial tray menu (updated dynamically after webview loads)
            let (panel_label, quit_label) = {
                let lang = sys_locale::get_locale().unwrap_or_else(|| "en".to_string());
                (
                    quick_panel_label_for_locale(&lang).to_string(),
                    quit_label_for_locale(&lang),
                )
            };

            // Create tray menu with a compact panel launcher and quit action.
            let panel = MenuItem::with_id(app, "open_panel", panel_label, true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", quit_label, true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&panel, &quit])?;

            // Build tray icon with template image (macOS auto-adapts for light/dark mode)
            let icon = Image::from_bytes(include_bytes!("../icons/tray.png"))
                .expect("Failed to load tray icon");
            let tray = TrayIconBuilder::new()
                .icon(icon)
                .icon_as_template(true)
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "open_panel" => {
                        info!("Quick panel requested from tray");
                        if let Err(e) = open_or_focus_panel_window(&app.app_handle()) {
                            error!("Failed to open quick panel: {}", e);
                        }
                    }
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
                        if let Err(e) = open_or_focus_main_window(&tray.app_handle()) {
                            error!("Failed to open main window from tray: {}", e);
                        }
                    }
                })
                .build(app)?;

            // Store tray icon in app state for cleanup on Windows
            app.manage(tray);
            app.manage(TrayPanelItem(panel));
            app.manage(TrayQuitItem(quit));
            DESKTOP_STARTUP_TRACE.mark("tray_ready");

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
                    DESKTOP_STARTUP_TRACE.mark("stt_auth_scheduled");
                    let stt_app_handle = app.handle().clone();
                    let _ = stt_app_handle.run_on_main_thread(move || {
                        DESKTOP_STARTUP_TRACE.mark("stt_auth_request_begin");
                        info!("Requesting macOS speech recognition authorization...");
                        let status = blue_ffi::request_stt_authorization();
                        match status {
                            3 => info!("Speech recognition authorized"),
                            1 => info!("Speech recognition denied by user"),
                            2 => info!("Speech recognition restricted"),
                            0 => info!("Speech recognition not determined"),
                            _ => info!("Speech recognition status: {}", status),
                        }
                        DESKTOP_STARTUP_TRACE.mark("stt_auth_request_complete");
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
                DESKTOP_STARTUP_TRACE.mark("server_task_enter");
                match start_server_platform(&app_handle).await {
                    Ok(_) => {
                        DESKTOP_STARTUP_TRACE.mark("server_start_complete");
                        info!("Server started successfully");
                    }
                    Err(e) => {
                        error!("Failed to start server: {}", e);
                        for label in [MAIN_WINDOW_LABEL, PANEL_WINDOW_LABEL] {
                            if let Some(window) = app_handle.get_webview_window(label) {
                                show_server_start_error(&window, &e);
                            }
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

                // Replace the startup splash with the real localhost UI as soon as the server is ready.
                DESKTOP_STARTUP_TRACE.mark("main_window_bind_begin");
                match bind_window_to_server_path(&app_handle_for_window, MAIN_WINDOW_LABEL, "") {
                    Ok(window) => {
                        DESKTOP_STARTUP_TRACE.mark("main_window_navigated");
                        #[cfg(target_os = "macos")]
                        update_window_on_main_thread(
                            &app_handle_for_window,
                            MAIN_WINDOW_LABEL,
                            true,
                            true,
                        );

                        let protocol = if use_https { "https" } else { "http" };
                        let url = format!("{}://localhost:{}", protocol, port);
                        info!("Main window bound to server at {}", url);
                        #[cfg(not(target_os = "macos"))]
                        show_and_focus_window(&window);

                        if app_handle_for_window
                            .get_webview_window(PANEL_WINDOW_LABEL)
                            .is_some()
                        {
                            match bind_window_to_server_path(
                                &app_handle_for_window,
                                PANEL_WINDOW_LABEL,
                                PANEL_WINDOW_PATH,
                            ) {
                                Ok(_panel_window) => {
                                    DESKTOP_STARTUP_TRACE.mark("panel_window_navigated");
                                    #[cfg(target_os = "macos")]
                                    update_window_on_main_thread(
                                        &app_handle_for_window,
                                        PANEL_WINDOW_LABEL,
                                        true,
                                        false,
                                    );
                                    #[cfg(not(target_os = "macos"))]
                                    let _ = _panel_window.show();
                                }
                                Err(e) => {
                                    error!("Failed to bind quick panel to server: {}", e);
                                }
                            }
                        }

                        // on_page_load shows the window as soon as the HTML loads (splash visible).
                        // Fallback: if frontend somehow fails, force-show after 5s.
                        DESKTOP_STARTUP_TRACE.mark("main_window_fallback_wait_begin");
                        tokio::time::sleep(std::time::Duration::from_millis(5000)).await;
                        DESKTOP_STARTUP_TRACE.mark("main_window_fallback_wait_complete");
                        if !window.is_visible().unwrap_or(true) {
                            DESKTOP_STARTUP_TRACE.mark("main_window_fallback_show");
                            info!("Fallback: showing window after timeout");
                            #[cfg(target_os = "macos")]
                            update_window_on_main_thread(
                                &app_handle_for_window,
                                MAIN_WINDOW_LABEL,
                                false,
                                true,
                            );
                            #[cfg(not(target_os = "macos"))]
                            let _ = window.show();
                            #[cfg(not(target_os = "macos"))]
                            let _ = window.set_focus();
                        }

                        // On macOS, activate the app to bring it to front
                        #[cfg(target_os = "macos")]
                        {
                            DESKTOP_STARTUP_TRACE.mark("main_window_activate_begin");
                            use std::process::Command;
                            let _ = Command::new("osascript")
                                .args(["-e", "tell application \"ZimaOS Blue\" to activate"])
                                .output();
                            DESKTOP_STARTUP_TRACE.mark("main_window_activate_complete");
                        }
                    }
                    Err(e) => {
                        error!("Failed to bind main window to server: {}", e);
                    }
                }
            });

            info!("Application setup complete");
            DESKTOP_STARTUP_TRACE.mark("setup_complete");
            Ok(())
        });

    DESKTOP_STARTUP_TRACE.mark("builder_configured");
    DESKTOP_STARTUP_TRACE.mark("build_start");

    let app = builder
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    DESKTOP_STARTUP_TRACE.mark("build_complete");

    app.run(|app_handle, event| {
        match event {
            RunEvent::ExitRequested { api, .. } => {
                // Only prevent exit if we're not actually quitting AND minimize-to-tray/menu-bar is enabled
                if !QUITTING.load(Ordering::SeqCst) && MINIMIZE_TO_TRAY.load(Ordering::SeqCst) {
                    api.prevent_exit();
                    for label in [MAIN_WINDOW_LABEL, PANEL_WINDOW_LABEL] {
                        if let Some(window) = app_handle.get_webview_window(label) {
                            let _ = window.hide();
                        }
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

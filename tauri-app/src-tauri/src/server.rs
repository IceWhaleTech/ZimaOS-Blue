// Server management module
// Handles starting, stopping, and monitoring the Echo Go server as a sidecar

use crate::AppState;
use log::{info, warn};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use std::time::Duration;
use tauri::{AppHandle, Manager};
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;
use tokio::sync::Mutex;
use tokio::time::sleep;

/// Server status information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerStatus {
    pub running: bool,
    pub port: u16,
    pub url: String,
    pub pid: Option<u32>,
    pub uptime_seconds: Option<u64>,
}

/// Global server process handle
static SERVER_PROCESS: once_cell::sync::Lazy<Arc<Mutex<Option<CommandChild>>>> =
    once_cell::sync::Lazy::new(|| Arc::new(Mutex::new(None)));

/// Start time for uptime calculation
static START_TIME: once_cell::sync::Lazy<Arc<Mutex<Option<std::time::Instant>>>> =
    once_cell::sync::Lazy::new(|| Arc::new(Mutex::new(None)));

/// Find an available port starting from the given port
async fn find_available_port(start_port: u16) -> u16 {
    for port in start_port..start_port + 100 {
        if std::net::TcpListener::bind(("127.0.0.1", port)).is_ok() {
            return port;
        }
    }
    start_port
}

/// Start the Echo server as a sidecar process
pub async fn start_sidecar_server(app: &AppHandle) -> Result<(), String> {
    info!("Starting Echo server sidecar");

    // Check if already running
    let mut process_guard = SERVER_PROCESS.lock().await;
    if process_guard.is_some() {
        warn!("Server is already running");
        return Ok(());
    }

    // Find available port
    let port = find_available_port(8080).await;
    info!("Using port {}", port);

    // Update app state
    if let Some(state) = app.try_state::<AppState>() {
        *state.server_port.lock().unwrap() = port;
    }

    // Get the sidecar command
    let shell = app.shell();
    let sidecar = shell
        .sidecar("echo-server")
        .map_err(|e| format!("Failed to create sidecar command: {}", e))?;

    // Start the server with port argument
    let (mut rx, child) = sidecar
        .args(["--port", &port.to_string()])
        .spawn()
        .map_err(|e| format!("Failed to spawn sidecar: {}", e))?;

    info!("Server process started");

    // Store the process handle
    *process_guard = Some(child);
    drop(process_guard);

    // Record start time
    *START_TIME.lock().await = Some(std::time::Instant::now());

    // Update running state
    if let Some(state) = app.try_state::<AppState>() {
        *state.server_running.lock().unwrap() = true;
    }

    // Spawn a task to monitor server output
    let app_handle = app.clone();
    tauri::async_runtime::spawn(async move {
        while let Some(event) = rx.recv().await {
            match event {
                tauri_plugin_shell::process::CommandEvent::Stdout(line) => {
                    info!("[server] {}", String::from_utf8_lossy(&line));
                }
                tauri_plugin_shell::process::CommandEvent::Stderr(line) => {
                    warn!("[server] {}", String::from_utf8_lossy(&line));
                }
                tauri_plugin_shell::process::CommandEvent::Terminated(payload) => {
                    info!("Server terminated with code: {:?}", payload.code);

                    // Update state
                    if let Some(state) = app_handle.try_state::<AppState>() {
                        *state.server_running.lock().unwrap() = false;
                    }

                    // Clear process handle
                    *SERVER_PROCESS.lock().await = None;
                    *START_TIME.lock().await = None;

                    break;
                }
                _ => {}
            }
        }
    });

    // Wait for server to be ready
    let url = format!("http://localhost:{}/api/v1/health", port);
    for i in 0..30 {
        sleep(Duration::from_millis(100)).await;
        if reqwest::get(&url).await.is_ok() {
            info!("Server is ready after {}ms", (i + 1) * 100);
            return Ok(());
        }
    }

    warn!("Server may not be fully ready, but process is running");
    Ok(())
}

/// Start the server (Tauri command)
#[tauri::command]
pub async fn start_server(app: AppHandle) -> Result<ServerStatus, String> {
    start_sidecar_server(&app).await?;
    get_server_status(app).await
}

/// Stop the server (Tauri command)
#[tauri::command]
pub async fn stop_server(app: AppHandle) -> Result<(), String> {
    info!("Stopping Echo server");

    let mut process_guard = SERVER_PROCESS.lock().await;
    if let Some(child) = process_guard.take() {
        child.kill().map_err(|e| format!("Failed to kill server: {}", e))?;
        info!("Server stopped");
    }

    // Update state
    if let Some(state) = app.try_state::<AppState>() {
        *state.server_running.lock().unwrap() = false;
    }

    *START_TIME.lock().await = None;

    Ok(())
}

/// Restart the server (Tauri command)
#[tauri::command]
pub async fn restart_server(app: AppHandle) -> Result<ServerStatus, String> {
    info!("Restarting Echo server");
    stop_server(app.clone()).await?;
    sleep(Duration::from_millis(500)).await;
    start_server(app).await
}

/// Get server status (Tauri command)
#[tauri::command]
pub async fn get_server_status(app: AppHandle) -> Result<ServerStatus, String> {
    let process_guard = SERVER_PROCESS.lock().await;
    let running = process_guard.is_some();
    drop(process_guard);

    let port = if let Some(state) = app.try_state::<AppState>() {
        *state.server_port.lock().unwrap()
    } else {
        8080
    };

    let uptime = if let Some(start) = *START_TIME.lock().await {
        Some(start.elapsed().as_secs())
    } else {
        None
    };

    Ok(ServerStatus {
        running,
        port,
        url: format!("http://localhost:{}", port),
        pid: None, // PID not easily accessible with current API
        uptime_seconds: uptime,
    })
}

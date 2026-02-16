// Server management module
// Handles starting, stopping, and monitoring the Echo Go server as a sidecar

use crate::AppState;
use log::{info, warn};
use serde::{Deserialize, Serialize};
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::time::Duration;
use tauri::{AppHandle, Manager};
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

/// Global server process handle (using std::process::Child)
static SERVER_PROCESS: once_cell::sync::Lazy<Arc<Mutex<Option<Child>>>> =
    once_cell::sync::Lazy::new(|| Arc::new(Mutex::new(None)));

/// Server process ID for tracking
static SERVER_PID: once_cell::sync::Lazy<Arc<Mutex<Option<u32>>>> =
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

/// Shared HTTP client for health checks (reuse connections, bypass proxy for localhost)
static HTTP_CLIENT: once_cell::sync::Lazy<reqwest::Client> = once_cell::sync::Lazy::new(|| {
    reqwest::Client::builder()
        .timeout(Duration::from_secs(1))
        .pool_max_idle_per_host(2)
        .no_proxy() // Bypass system proxy for localhost connections
        .danger_accept_invalid_certs(true) // Trust self-signed certificates for localhost
        .build()
        .unwrap_or_else(|_| reqwest::Client::new())
});

/// Check if an existing Echo server is healthy on the given port
async fn check_existing_server(port: u16) -> bool {
    let url = format!("http://localhost:{}/api/v1/health", port);
    match HTTP_CLIENT
        .get(&url)
        .timeout(Duration::from_millis(500))
        .send()
        .await
    {
        Ok(resp) => resp.status().is_success(),
        Err(_) => false,
    }
}

/// Kill any existing blue-server processes
fn kill_existing_echo_servers() {
    info!("Checking for existing blue-server processes");

    #[cfg(unix)]
    {
        let _ = Command::new("pkill")
            .args(["-f", "blue-server"])
            .output();
        std::thread::sleep(Duration::from_millis(100));
    }

    #[cfg(windows)]
    {
        let _ = Command::new("taskkill")
            .args(["/F", "/IM", "blue-server.exe"])
            .output();
        std::thread::sleep(Duration::from_millis(100));
    }
}

/// Get the sidecar binary path
fn get_sidecar_path() -> Result<std::path::PathBuf, String> {
    let exe_path = std::env::current_exe()
        .map_err(|e| format!("Failed to get current exe path: {}", e))?;
    let exe_dir = exe_path
        .parent()
        .ok_or_else(|| "Failed to get exe directory".to_string())?;

    let sidecar_name = if cfg!(target_os = "windows") {
        "blue-server.exe"
    } else {
        "blue-server"
    };

    let sidecar_path = exe_dir.join(sidecar_name);

    if sidecar_path.exists() {
        return Ok(sidecar_path);
    }

    // Try alternative locations
    // On macOS, might be in Resources
    #[cfg(target_os = "macos")]
    {
        if let Some(parent) = exe_dir.parent() {
            let resources_path = parent.join("Resources").join(sidecar_name);
            if resources_path.exists() {
                return Ok(resources_path);
            }
        }
    }

    Err(format!("Sidecar binary not found. Checked: {:?}", sidecar_path))
}

/// Start the Echo server as a sidecar process
pub async fn start_sidecar_server(app: &AppHandle) -> Result<(), String> {
    info!("Starting Echo server sidecar");

    // Check if already running (managed by this process)
    let mut process_guard = SERVER_PROCESS.lock().await;
    if process_guard.is_some() {
        warn!("Server is already running");
        return Ok(());
    }

    let default_port = 23456u16;

    // Check if there's an existing healthy server on port 23456
    if check_existing_server(default_port).await {
        info!(
            "Found existing healthy Echo server on port {}, reusing it",
            default_port
        );

        // Update app state to use existing server
        if let Some(state) = app.try_state::<AppState>() {
            *state.server_port.lock().unwrap() = default_port;
            *state.server_running.lock().unwrap() = true;
        }

        // Record start time (approximate)
        *START_TIME.lock().await = Some(std::time::Instant::now());

        return Ok(());
    }

    // Check if port 23456 is occupied but server is not healthy (zombie process)
    if std::net::TcpListener::bind(("127.0.0.1", default_port)).is_err() {
        warn!(
            "Port {} is occupied but server is not healthy, killing existing processes",
            default_port
        );
        kill_existing_echo_servers();
        sleep(Duration::from_millis(100)).await;
    }

    // Find available port
    let port = find_available_port(default_port).await;
    info!("Using port {}", port);

    // Update app state
    if let Some(state) = app.try_state::<AppState>() {
        *state.server_port.lock().unwrap() = port;
    }

    // Get the sidecar binary path
    let sidecar_path = get_sidecar_path()?;
    info!("Found sidecar at: {:?}", sidecar_path);

    // Start the sidecar using std::process::Command
    // Don't pipe stdout/stderr to avoid blocking when buffer fills up
    let mut cmd = Command::new(&sidecar_path);
    cmd.env("ECHO_SERVER_PORT", port.to_string())
        .stdout(Stdio::null())
        .stderr(Stdio::null());

    // On Windows, hide the console window
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x08000000;
        cmd.creation_flags(CREATE_NO_WINDOW);
    }

    let child = cmd.spawn()
        .map_err(|e| format!("Failed to spawn sidecar: {}", e))?;

    let pid = child.id();
    info!("Sidecar process started with PID: {}", pid);

    // Store the process handle and PID
    *process_guard = Some(child);
    *SERVER_PID.lock().await = Some(pid);
    drop(process_guard);

    // Record start time
    *START_TIME.lock().await = Some(std::time::Instant::now());

    // Update running state
    if let Some(state) = app.try_state::<AppState>() {
        *state.server_running.lock().unwrap() = true;
    }

    // Wait for server to be ready with exponential backoff (optimized for faster startup)
    let url = format!("http://localhost:{}/api/v1/health", port);
    let mut delay_ms = 20u64;   // 优化: 更快的首次检查 (20ms)
    let max_delay_ms = 100u64;  // 优化: 最大延迟 100ms
    let max_attempts = 6;       // 优化: 6 次尝试 (~200ms 总时间)

    for i in 0..max_attempts {
        sleep(Duration::from_millis(delay_ms)).await;
        if HTTP_CLIENT.get(&url).send().await.is_ok() {
            info!(
                "Server is ready after attempt {} (~{}ms total)",
                i + 1,
                (0..=i)
                    .map(|j| std::cmp::min(20 * 2u64.pow(j as u32), max_delay_ms))
                    .sum::<u64>()
            );
            return Ok(());
        }
        delay_ms = std::cmp::min(delay_ms * 2, max_delay_ms);
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
    if let Some(mut child) = process_guard.take() {
        // Try to kill the process
        if let Err(e) = child.kill() {
            warn!("Failed to kill server process: {}", e);
        }
        // Wait for it to exit
        let _ = child.wait();
        info!("Server stopped");
    }

    // Update state
    if let Some(state) = app.try_state::<AppState>() {
        *state.server_running.lock().unwrap() = false;
    }

    *SERVER_PID.lock().await = None;
    *START_TIME.lock().await = None;

    Ok(())
}

/// Restart the server (Tauri command)
#[tauri::command]
pub async fn restart_server(app: AppHandle) -> Result<ServerStatus, String> {
    info!("Restarting Echo server");
    stop_server(app.clone()).await?;
    sleep(Duration::from_millis(200)).await;
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
        23456
    };

    let uptime = if let Some(start) = *START_TIME.lock().await {
        Some(start.elapsed().as_secs())
    } else {
        None
    };

    let pid = *SERVER_PID.lock().await;

    Ok(ServerStatus {
        running,
        port,
        url: format!("http://localhost:{}", port),
        pid,
        uptime_seconds: uptime,
    })
}

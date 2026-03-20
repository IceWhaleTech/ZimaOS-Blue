#[path = "../src/blue_ffi.rs"]
mod blue_ffi;

use clap::Parser;
use reqwest::Client;
use std::path::PathBuf;
use std::time::{Duration, Instant};
use tokio::time::sleep;

#[derive(Parser, Debug)]
#[command(
    name = "embedded-startup-probe",
    about = "Profile embedded Blue server startup (stops the server on exit unless --keep-running is set)"
)]
struct Args {
    #[arg(long, default_value_t = 0)]
    port: u16,

    #[arg(long)]
    data_dir: Option<PathBuf>,

    #[arg(long)]
    extra_args: Option<String>,

    #[arg(long, default_value_t = 8000)]
    timeout_ms: u64,

    #[arg(long, default_value_t = false)]
    keep_running: bool,
}

fn default_probe_data_dir() -> Option<PathBuf> {
    #[cfg(target_os = "macos")]
    {
        return dirs::home_dir().map(|home| home.join(".zimaos-blue").join("data"));
    }

    #[cfg(target_os = "windows")]
    {
        return std::env::current_exe()
            .ok()
            .and_then(|exe| exe.parent().map(|p| p.join("data")))
            .or_else(|| dirs::home_dir().map(|home| home.join(".zimaos-blue").join("data")));
    }

    #[cfg(not(any(target_os = "macos", target_os = "windows")))]
    {
        None
    }
}

struct ServerCleanup {
    keep_running: bool,
}

impl Drop for ServerCleanup {
    fn drop(&mut self) {
        if self.keep_running {
            return;
        }
        eprintln!(
            "probe completed successfully; stopping embedded server on exit (pass --keep-running to keep it running)"
        );
        let _ = blue_ffi::stop_server();
        blue_ffi::cleanup();
    }
}

async fn wait_for_port(timeout: Duration) -> Result<u16, String> {
    let started = Instant::now();
    loop {
        let port = blue_ffi::get_port();
        if port != 0 {
            return Ok(port);
        }
        if started.elapsed() >= timeout {
            return Err("timed out waiting for embedded server port".to_string());
        }
        sleep(Duration::from_millis(10)).await;
    }
}

async fn wait_for_health(client: &Client, port: u16, timeout: Duration) -> Result<(), String> {
    let started = Instant::now();
    let url = format!("http://127.0.0.1:{port}/api/v1/health");
    loop {
        match client.get(&url).send().await {
            Ok(response) if response.status().is_success() => return Ok(()),
            Ok(_) | Err(_) => {}
        }
        if started.elapsed() >= timeout {
            return Err("timed out waiting for embedded server health".to_string());
        }
        sleep(Duration::from_millis(20)).await;
    }
}

async fn fetch_and_report(
    client: &Client,
    started: Instant,
    port: u16,
    path: &str,
    label: &str,
) -> Result<(), String> {
    let url = format!("http://127.0.0.1:{port}{path}");
    let request_started = Instant::now();
    let response = client
        .get(&url)
        .send()
        .await
        .map_err(|err| format!("request failed for {path}: {err}"))?;
    let status = response.status().as_u16();
    let body = response
        .bytes()
        .await
        .map_err(|err| format!("failed reading body for {path}: {err}"))?;

    println!(
        "mark={} total_ms={} request_ms={} status={} bytes={}",
        label,
        started.elapsed().as_millis(),
        request_started.elapsed().as_millis(),
        status,
        body.len()
    );
    Ok(())
}

#[tokio::main(flavor = "current_thread")]
async fn main() -> Result<(), String> {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info"))
        .format_timestamp(None)
        .format_module_path(false)
        .format_target(false)
        .format_level(false)
        .init();

    let args = Args::parse();
    let timeout = Duration::from_millis(args.timeout_ms);
    let data_dir = args.data_dir.clone().or_else(default_probe_data_dir);
    let data_dir_display = data_dir
        .as_ref()
        .map(|path| path.to_string_lossy().into_owned());
    let started = Instant::now();

    println!("mark=probe_start total_ms=0");
    if args.data_dir.is_none() {
        if let Some(dir) = data_dir.as_ref() {
            eprintln!(
                "probe using desktop-compatible default data_dir: {}",
                dir.display()
            );
        }
    }
    blue_ffi::start_server_with_args(
        args.port,
        data_dir_display.as_deref(),
        args.extra_args.as_deref(),
    )?;
    println!(
        "mark=ffi_start_returned total_ms={}",
        started.elapsed().as_millis()
    );

    let _cleanup = ServerCleanup {
        keep_running: args.keep_running,
    };

    let actual_port = wait_for_port(timeout).await?;
    println!(
        "mark=embedded_port_ready total_ms={} actual_port={}",
        started.elapsed().as_millis(),
        actual_port
    );

    let client = Client::builder()
        .danger_accept_invalid_certs(true)
        .no_proxy()
        .timeout(Duration::from_millis(800))
        .build()
        .map_err(|err| format!("failed to build probe client: {err}"))?;

    wait_for_health(&client, actual_port, timeout).await?;
    println!(
        "mark=embedded_health_ready total_ms={}",
        started.elapsed().as_millis()
    );

    fetch_and_report(
        &client,
        started,
        actual_port,
        "/api/v1/health",
        "embedded_health_fetch",
    )
    .await?;
    fetch_and_report(&client, started, actual_port, "/", "embedded_index_fetch").await?;

    println!(
        "mark=probe_complete total_ms={} action={}",
        started.elapsed().as_millis(),
        if args.keep_running {
            "keep_running"
        } else {
            "stop_on_exit"
        }
    );

    Ok(())
}

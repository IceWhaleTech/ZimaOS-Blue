#[cfg(target_os = "windows")]
use std::ffi::OsString;
#[cfg(target_os = "windows")]
use std::path::PathBuf;
#[cfg(target_os = "windows")]
use windows_service::{
    service::{
        ServiceAccess, ServiceErrorControl, ServiceInfo, ServiceStartType, ServiceState,
        ServiceType,
    },
    service_manager::{ServiceManager, ServiceManagerAccess},
};

#[cfg(target_os = "windows")]
const SERVICE_NAME: &str = "ZimaOSBlue";
#[cfg(target_os = "windows")]
const SERVICE_DISPLAY_NAME: &str = "ZimaOS Blue";

#[cfg(target_os = "windows")]
pub fn install_service() -> Result<String, String> {
    let manager = ServiceManager::local_computer(
        None::<&str>,
        ServiceManagerAccess::CREATE_SERVICE,
    )
    .map_err(|e| format!("Failed to open service manager: {}", e))?;

    let exe_path = std::env::current_exe()
        .map_err(|e| format!("Failed to get executable path: {}", e))?;

    let service_info = ServiceInfo {
        name: OsString::from(SERVICE_NAME),
        display_name: OsString::from(SERVICE_DISPLAY_NAME),
        service_type: ServiceType::OWN_PROCESS,
        start_type: ServiceStartType::AutoStart,
        error_control: ServiceErrorControl::Normal,
        executable_path: exe_path,
        launch_arguments: vec![OsString::from("--service")],
        dependencies: vec![],
        account_name: None,
        account_password: None,
    };

    let _service = manager
        .create_service(&service_info, ServiceAccess::CHANGE_CONFIG)
        .map_err(|e| format!("Failed to create service: {}", e))?;

    Ok(format!("Service '{}' installed successfully", SERVICE_DISPLAY_NAME))
}

#[cfg(target_os = "windows")]
pub fn uninstall_service() -> Result<String, String> {
    let manager = ServiceManager::local_computer(
        None::<&str>,
        ServiceManagerAccess::CONNECT,
    )
    .map_err(|e| format!("Failed to open service manager: {}", e))?;

    let service = manager
        .open_service(SERVICE_NAME, ServiceAccess::DELETE | ServiceAccess::QUERY_STATUS)
        .map_err(|e| format!("Failed to open service: {}", e))?;

    // Stop service if running
    let status = service.query_status()
        .map_err(|e| format!("Failed to query service status: {}", e))?;

    if status.current_state != ServiceState::Stopped {
        service.stop()
            .map_err(|e| format!("Failed to stop service: {}", e))?;
    }

    service.delete()
        .map_err(|e| format!("Failed to delete service: {}", e))?;

    Ok(format!("Service '{}' uninstalled successfully", SERVICE_DISPLAY_NAME))
}

#[cfg(not(target_os = "windows"))]
pub fn install_service() -> Result<String, String> {
    Err("Service installation only supported on Windows".to_string())
}

#[cfg(not(target_os = "windows"))]
pub fn uninstall_service() -> Result<String, String> {
    Err("Service uninstallation only supported on Windows".to_string())
}

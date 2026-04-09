// Enhanced tray menu module with better styling and functionality
// This module provides an improved tray menu experience for Windows

use tauri::{
    image::Image,
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Manager,
};
use log::info;

/// Build an enhanced tray menu with better styling
pub fn build_tray_menu(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    // Create menu items with emoji icons for better visual appeal
    let show = MenuItem::with_id(app, "show", "🪟 Show Window", true, None::<&str>)?;
    let hide = MenuItem::with_id(app, "hide", "🙈 Hide Window", true, None::<&str>)?;
    let separator1 = MenuItem::with_id(app, "sep1", "", false, None::<&str>)?;
    let restart = MenuItem::with_id(app, "restart", "🔄 Restart Server", true, None::<&str>)?;
    let separator2 = MenuItem::with_id(app, "sep2", "", false, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "❌ Exit", true, None::<&str>)?;

    let menu = Menu::with_items(app, &[&show, &hide, &separator1, &restart, &separator2, &quit])?;

    // Load tray icon
    let icon = Image::from_bytes(include_bytes!("../nsis/icons/tray.png"))
        .expect("Failed to load tray icon");

    // Build and configure tray icon
    let _tray = TrayIconBuilder::new()
        .icon(icon)
        .menu(&menu)
        .show_menu_on_left_click(false)
        .tooltip("ZimaOS Blue - AI Gateway")
        .on_menu_event(|app, event| {
            match event.id.as_ref() {
                "show" => {
                    info!("Show window requested from tray");
                    if let Some(window) = app.get_webview_window("main") {
                        let _ = window.show();
                        let _ = window.set_focus();
                    }
                }
                "hide" => {
                    info!("Hide window requested from tray");
                    if let Some(window) = app.get_webview_window("main") {
                        let _ = window.hide();
                    }
                }
                "restart" => {
                    info!("Restart server requested from tray");
                    // Emit event to frontend to restart server
                    let _ = app.emit("restart-server", ());
                }
                "quit" => {
                    info!("Quit requested from tray");
                    app.exit(0);
                }
                _ => {}
            }
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

    Ok(())
}

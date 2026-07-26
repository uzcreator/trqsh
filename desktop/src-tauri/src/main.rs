// trqsh desktop shell.
//
// This is a thin native client over the Go agent. The Go agent ships as a
// bundled sidecar binary (bundle.externalBin = "binaries/trqsh"); on startup we
// spawn `trqsh daemon`, which serves the loopback control API on 127.0.0.1:4041
// and writes a bearer token to ~/.trqsh/control.token. The WebView UI reads the
// endpoint via get_agent_endpoint() and drives the agent over plain HTTP + SSE.
//
// The UI is never in the tunnel data path (internet → edge → agent → localhost),
// so the shell has zero effect on tunnel throughput or server scale — those live
// entirely in the Go engine.

// No console window on Windows release builds.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::path::PathBuf;
use std::sync::Mutex;

use serde::Serialize;
use tauri::{Manager, State};
use tauri_plugin_opener::OpenerExt;
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;

// The daemon's fixed loopback control address (mirrors the Go default).
const CONTROL_BASE: &str = "http://127.0.0.1:4041";

#[derive(Serialize)]
struct Endpoint {
    base: String,
    token: String,
}

#[derive(Serialize)]
struct HostInfo {
    os: String,
    arch: String,
    version: String,
    dashboard_url: String,
    keys_url: String,
    billing_url: String,
    docs_url: String,
}

/// Handle to the running Go agent sidecar so we can stop it on exit.
#[derive(Default)]
struct Sidecar(Mutex<Option<CommandChild>>);

fn control_token_path(app: &tauri::AppHandle) -> Option<PathBuf> {
    app.path()
        .home_dir()
        .ok()
        .map(|h| h.join(".trqsh").join("control.token"))
}

/// Returns the loopback control API base URL and the bearer token the daemon
/// wrote. The token file appears once the daemon has started; until then the
/// token is empty and the first calls 401 (the UI retries as the agent comes up).
#[tauri::command]
fn get_agent_endpoint(app: tauri::AppHandle) -> Endpoint {
    let token = control_token_path(&app)
        .and_then(|p| std::fs::read_to_string(p).ok())
        .map(|s| s.trim().to_string())
        .unwrap_or_default();
    Endpoint {
        base: CONTROL_BASE.to_string(),
        token,
    }
}

#[tauri::command]
fn get_host_info() -> HostInfo {
    HostInfo {
        os: std::env::consts::OS.to_string(),
        arch: std::env::consts::ARCH.to_string(),
        version: env!("CARGO_PKG_VERSION").to_string(),
        dashboard_url: "https://dashboard.trqsh.uz".into(),
        keys_url: "https://dashboard.trqsh.uz/keys".into(),
        billing_url: "https://dashboard.trqsh.uz/billing".into(),
        docs_url: "https://trqsh.uz/docs".into(),
    }
}

/// Opens a URL in the user's default browser (external links only).
#[tauri::command]
fn open_url(app: tauri::AppHandle, url: String) -> Result<(), String> {
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn quit(app: tauri::AppHandle) {
    app.exit(0);
}

/// Spawns the bundled Go agent as a background daemon.
fn spawn_agent(app: &tauri::AppHandle) -> Result<CommandChild, String> {
    let cmd = app
        .shell()
        .sidecar("binaries/trqsh")
        .map_err(|e| e.to_string())?;
    let (_rx, child) = cmd.args(["daemon"]).spawn().map_err(|e| e.to_string())?;
    Ok(child)
}

fn stop_agent(app: &tauri::AppHandle) {
    if let Some(state) = app.try_state::<Sidecar>() {
        if let Ok(mut guard) = state.0.lock() {
            if let Some(child) = guard.take() {
                let _ = child.kill();
            }
        }
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_opener::init())
        .manage(Sidecar::default())
        .invoke_handler(tauri::generate_handler![
            get_agent_endpoint,
            get_host_info,
            open_url,
            quit
        ])
        .setup(|app| {
            let handle = app.handle().clone();
            match spawn_agent(&handle) {
                Ok(child) => {
                    let state: State<Sidecar> = app.state();
                    *state.0.lock().unwrap() = Some(child);
                }
                Err(e) => eprintln!("trqsh: failed to start agent sidecar: {e}"),
            }
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building trqsh desktop")
        // Reap the agent sidecar whenever the app is exiting, so quitting (tray,
        // window close, or the quit command) never leaves an orphan daemon.
        .run(|app_handle, event| {
            if matches!(
                event,
                tauri::RunEvent::ExitRequested { .. } | tauri::RunEvent::Exit
            ) {
                stop_agent(app_handle);
            }
        });
}

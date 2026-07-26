// trqsh desktop shell.
//
// This is a thin native client over the Go agent. The Go agent ships as a
// bundled binary (bundle.externalBin = "binaries/trqsh") that Tauri installs
// next to the app executable as `trqsh` (triple stripped). On startup we spawn
// `trqsh daemon`, which serves the loopback control API on 127.0.0.1:4041 and
// writes a bearer token to ~/.trqsh/control.token. The WebView UI reads the
// endpoint via get_agent_endpoint() and drives the agent over plain HTTP + SSE.
//
// The UI is never in the tunnel data path (internet → edge → agent → localhost),
// so the shell has zero effect on tunnel throughput or server scale — those live
// entirely in the Go engine.
//
// We spawn the agent with std::process directly against the resolved path next
// to the app executable, rather than Tauri's sidecar resolver, so there is no
// ambiguity about the bundled path — and we log the outcome to ~/.trqsh so
// startup problems are diagnosable in a windowed (no-console) build.

// No console window on Windows release builds.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::io::Write;
use std::path::PathBuf;
use std::process::{Child, Command};
use std::sync::Mutex;

use serde::Serialize;
use tauri::Manager;
use tauri_plugin_opener::OpenerExt;

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

/// Handle to the running Go agent so we can stop it on exit.
#[derive(Default)]
struct Sidecar(Mutex<Option<Child>>);

fn trqsh_dir(app: &tauri::AppHandle) -> Option<PathBuf> {
    app.path().home_dir().ok().map(|h| h.join(".trqsh"))
}

fn control_token_path(app: &tauri::AppHandle) -> Option<PathBuf> {
    trqsh_dir(app).map(|d| d.join("control.token"))
}

/// Appends a line to ~/.trqsh/desktop.log. Startup runs in a windowed process
/// with no console, so this file is how we surface agent-spawn problems.
fn log_line(app: &tauri::AppHandle, msg: &str) {
    if let Some(dir) = trqsh_dir(app) {
        let _ = std::fs::create_dir_all(&dir);
        if let Ok(mut f) = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(dir.join("desktop.log"))
        {
            let _ = writeln!(f, "{msg}");
        }
    }
}

/// Resolves the bundled agent binary next to the app executable.
fn agent_binary() -> Option<PathBuf> {
    let name = if cfg!(windows) { "trqsh.exe" } else { "trqsh" };
    let exe = std::env::current_exe().ok()?;
    let candidate = exe.parent()?.join(name);
    candidate.exists().then_some(candidate)
}

/// Returns the loopback control API base URL and the bearer token the daemon
/// wrote. The token file appears once the daemon has started; until then the
/// token is empty and the UI retries as the agent comes up.
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
        dashboard_url: "https://app.trqsh.uz".into(),
        keys_url: "https://app.trqsh.uz/keys".into(),
        billing_url: "https://app.trqsh.uz/billing".into(),
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
fn spawn_agent(app: &tauri::AppHandle) -> std::io::Result<Child> {
    let bin = agent_binary().ok_or_else(|| {
        std::io::Error::new(
            std::io::ErrorKind::NotFound,
            "agent binary (trqsh) not found next to the app executable",
        )
    })?;
    log_line(app, &format!("spawning agent: {}", bin.display()));
    let mut cmd = Command::new(&bin);
    cmd.arg("daemon");
    // Don't pop a console window on Windows (CREATE_NO_WINDOW).
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(0x0800_0000);
    }
    cmd.spawn()
}

fn stop_agent(app: &tauri::AppHandle) {
    if let Some(state) = app.try_state::<Sidecar>() {
        if let Ok(mut guard) = state.0.lock() {
            if let Some(mut child) = guard.take() {
                let _ = child.kill();
            }
        }
    }
}

fn main() {
    tauri::Builder::default()
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
                    log_line(&handle, &format!("agent started, pid={}", child.id()));
                    *handle.state::<Sidecar>().0.lock().unwrap() = Some(child);
                }
                Err(e) => log_line(&handle, &format!("agent spawn FAILED: {e}")),
            }
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building trqsh desktop")
        // Reap the agent whenever the app exits, so quitting never leaves an
        // orphan daemon holding the control port.
        .run(|app_handle, event| {
            if matches!(
                event,
                tauri::RunEvent::ExitRequested { .. } | tauri::RunEvent::Exit
            ) {
                stop_agent(app_handle);
            }
        });
}

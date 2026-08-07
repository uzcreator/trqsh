// PTY session backend for the embedded terminal (VS Code-style, Ctrl+`).
//
// Spawns a real OS shell per session via portable-pty (cross-platform,
// includes Windows ConPTY) — not tauri-plugin-shell, which only runs a single
// command to completion and has no interactive stdin/stdout stream.
//
// Security: argv only, never shell-string interpolation. default_shell()
// resolves from a small fixed set of known binary names (or $SHELL on unix),
// never from remote/agent-sourced data — this hands the user their own
// shell, the same trust level as them opening a terminal themselves, not a
// privilege escalation. The four commands below are the entire surface (no
// generic "run a command" primitive), registered directly like every other
// command in main.rs — see capabilities/default.json for why that's the
// actual boundary here, not a capability-file entry.
//
// Output streams as raw bytes (Vec<u8>), not String: a multi-byte UTF-8
// sequence or ANSI escape can straddle two 4096-byte reads, and decoding
// each chunk independently would corrupt it. xterm.js (the frontend
// consumer) carries its own streaming UTF-8 decoder for exactly this, so
// forwarding raw bytes end-to-end is what actually round-trips correctly.

use std::collections::HashMap;
use std::io::{Read, Write};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;

// Every trait here is imported not just for its type name but because
// calling its methods via dot-syntax on a `Box<dyn Trait>` value requires
// the trait itself in scope — including ChildKiller, which is where `.kill()`
// actually lives (Child's supertrait), not on Child directly.
use portable_pty::{
    native_pty_system, Child, ChildKiller, CommandBuilder, MasterPty, PtySize, PtySystem, SlavePty,
};
use tauri::ipc::Channel;
use tauri::{AppHandle, Manager, State};

struct PtySession {
    master: Box<dyn MasterPty + Send>,
    writer: Box<dyn Write + Send>,
    child: Box<dyn Child + Send + Sync>,
}

#[derive(Default)]
pub struct PtyState(Mutex<HashMap<String, PtySession>>);

static NEXT_ID: AtomicU64 = AtomicU64::new(1);

/// Resolves the interactive shell to spawn. Windows: pwsh.exe if present,
/// else powershell.exe. Unix: $SHELL, else /bin/bash. Deliberately never
/// derived from anything remote or agent-sourced.
fn default_shell() -> String {
    #[cfg(windows)]
    {
        if which_exists("pwsh.exe") {
            "pwsh.exe".to_string()
        } else {
            "powershell.exe".to_string()
        }
    }
    #[cfg(not(windows))]
    {
        std::env::var("SHELL").unwrap_or_else(|_| "/bin/bash".to_string())
    }
}

#[cfg(windows)]
fn which_exists(name: &str) -> bool {
    // `where` is a standard Windows utility (distinct from a shell); the
    // argument is a hardcoded literal, never interpolated user input.
    std::process::Command::new("where")
        .arg(name)
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

/// Home directory fallback for a session's cwd, reusing the same
/// app.path().home_dir() call main.rs already makes for ~/.trqsh.
fn home_dir(app: &AppHandle) -> Option<String> {
    app.path()
        .home_dir()
        .ok()
        .map(|p| p.to_string_lossy().into_owned())
}

/// Spawns a new PTY session running the default shell. Streams its output
/// over `on_data` as raw byte chunks and returns a session id for
/// write/resize/kill. `cwd` falls back to the user's home directory.
#[tauri::command]
pub fn pty_spawn(
    app: AppHandle,
    state: State<'_, PtyState>,
    cwd: Option<String>,
    rows: u16,
    cols: u16,
    on_data: Channel<Vec<u8>>,
) -> Result<String, String> {
    let pty_system = native_pty_system();
    let pair = pty_system
        .openpty(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| e.to_string())?;

    let mut builder = CommandBuilder::new(default_shell());
    if let Some(dir) = cwd.filter(|s| !s.is_empty()).or_else(|| home_dir(&app)) {
        builder.cwd(dir);
    }

    let child = pair
        .slave
        .spawn_command(builder)
        .map_err(|e| e.to_string())?;
    // Drop the slave side once spawned: on Unix, the parent holding it open
    // can prevent the master's reader from ever seeing EOF after the child
    // exits.
    drop(pair.slave);

    let mut reader = pair.master.try_clone_reader().map_err(|e| e.to_string())?;
    let writer = pair.master.take_writer().map_err(|e| e.to_string())?;

    let id = format!("pty-{}", NEXT_ID.fetch_add(1, Ordering::Relaxed));

    std::thread::spawn(move || {
        let mut buf = [0u8; 4096];
        loop {
            match reader.read(&mut buf) {
                Ok(0) => break,
                Ok(n) => {
                    if on_data.send(buf[..n].to_vec()).is_err() {
                        break; // frontend gone / channel closed
                    }
                }
                Err(_) => break,
            }
        }
    });

    state.0.lock().unwrap().insert(
        id.clone(),
        PtySession {
            master: pair.master,
            writer,
            child,
        },
    );

    Ok(id)
}

/// Writes raw keystrokes/paste data to a session's shell.
#[tauri::command]
pub fn pty_write(state: State<'_, PtyState>, id: String, data: String) -> Result<(), String> {
    let mut sessions = state.0.lock().unwrap();
    let session = sessions.get_mut(&id).ok_or("unknown terminal session")?;
    session
        .writer
        .write_all(data.as_bytes())
        .map_err(|e| e.to_string())?;
    session.writer.flush().map_err(|e| e.to_string())
}

/// Notifies the kernel-level PTY of a new size, so full-screen/TUI programs
/// (vim, htop) redraw correctly instead of rendering garbled.
#[tauri::command]
pub fn pty_resize(
    state: State<'_, PtyState>,
    id: String,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    let sessions = state.0.lock().unwrap();
    let session = sessions.get(&id).ok_or("unknown terminal session")?;
    session
        .master
        .resize(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| e.to_string())
}

/// Kills a session's shell process and drops its PTY handles.
#[tauri::command]
pub fn pty_kill(state: State<'_, PtyState>, id: String) -> Result<(), String> {
    let mut sessions = state.0.lock().unwrap();
    if let Some(mut session) = sessions.remove(&id) {
        let _ = session.child.kill();
    }
    Ok(())
}

/// Kills every live session. Called from main.rs's RunEvent::Exit handler
/// alongside stop_agent, so quitting never leaves an orphan shell running.
pub fn stop_all(state: &PtyState) {
    if let Ok(mut sessions) = state.0.lock() {
        for (_, mut session) in sessions.drain() {
            let _ = session.child.kill();
        }
    }
}

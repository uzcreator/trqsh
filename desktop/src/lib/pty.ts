// Typed client over the Rust PTY backend (src-tauri/src/pty.rs) — spawns a
// real OS shell per session and streams its output as raw bytes over a
// Tauri Channel. Mirrors lib/agent.ts's dynamic-import pattern so
// browser-only `pnpm dev` (no Tauri) never crashes trying to import
// @tauri-apps/api; ptyClient is unusable there (see hasTauri() gate below,
// enforced by the terminal panel itself, not here).
//
// A Rust Vec<u8> crosses the IPC boundary as plain JSON, i.e. a JS
// number[], not a Uint8Array — there's no binary fast path here. Every
// method below converts to Uint8Array at the boundary, once, so callers
// (terminal-panel.tsx / xterm's own write()) only ever see real bytes.

import { hasTauri } from "./agent";

export interface PtySpawnOptions {
  cwd?: string;
  rows: number;
  cols: number;
}

export interface PtyClient {
  /** Spawns a shell session, streaming its output to `onData`. Returns a
   *  session id for write/resize/kill. */
  spawn(opts: PtySpawnOptions, onData: (bytes: Uint8Array) => void): Promise<string>;
  write(id: string, data: string): Promise<void>;
  /** cols/rows order matches xterm's own Terminal.resize(cols, rows) — the
   *  opposite of the Rust command's (id, rows, cols) parameter order. */
  resize(id: string, cols: number, rows: number): Promise<void>;
  kill(id: string): Promise<void>;
}

async function core() {
  return import("@tauri-apps/api/core");
}

export const ptyClient: PtyClient = {
  async spawn(opts, onData) {
    const { invoke, Channel } = await core();
    const channel = new Channel<number[]>();
    channel.onmessage = (bytes) => onData(new Uint8Array(bytes));
    return invoke<string>("pty_spawn", {
      cwd: opts.cwd,
      rows: opts.rows,
      cols: opts.cols,
      onData: channel,
    });
  },

  async write(id, data) {
    const { invoke } = await core();
    await invoke("pty_write", { id, data });
  },

  async resize(id, cols, rows) {
    const { invoke } = await core();
    await invoke("pty_resize", { id, rows, cols });
  },

  async kill(id) {
    const { invoke } = await core();
    await invoke("pty_kill", { id });
  },
};

export { hasTauri };

import { useCallback, useEffect, useRef, useState } from "react";
import { Plus, TerminalSquare, X } from "lucide-react";
import type { ITheme, Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { hasTauri, ptyClient } from "@/lib/pty";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

const MIN_HEIGHT = 120;
const DEFAULT_HEIGHT = 240;

/** Reads the app's own RGB-channel CSS vars (see index.css) and maps them to
 *  an xterm palette, so the terminal reads as part of this app's instrument
 *  voice instead of a generic black box. `--x --y --z` -> `rgb(x,y,z[,a])`. */
function cssVar(name: string, alpha?: number): string {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  if (!raw) return "";
  const channels = raw.split(/\s+/).join(",");
  return alpha === undefined ? `rgb(${channels})` : `rgba(${channels},${alpha})`;
}

function xtermTheme(): ITheme {
  return {
    background: cssVar("--page"),
    foreground: cssVar("--foreground"),
    cursor: cssVar("--primary"),
    cursorAccent: cssVar("--page"),
    selectionBackground: cssVar("--primary", 0.35),
    black: cssVar("--page"),
    red: cssVar("--critical"),
    green: cssVar("--good"),
    yellow: cssVar("--warning"),
    blue: cssVar("--wire"),
    magenta: cssVar("--serious"),
    cyan: cssVar("--wire"),
    white: cssVar("--foreground"),
    brightBlack: cssVar("--muted-ink"),
    brightRed: cssVar("--critical"),
    brightGreen: cssVar("--good"),
    brightYellow: cssVar("--warning"),
    brightBlue: cssVar("--wire"),
    brightMagenta: cssVar("--serious"),
    brightCyan: cssVar("--wire"),
    brightWhite: cssVar("--foreground"),
  };
}

interface Session {
  id: string;
  label: string;
}

/** One shell session: owns its xterm instance and PTY for its whole
 *  lifetime. Visibility is a CSS class (`hidden`), never an unmount —
 *  unmounting would destroy scrollback and force a respawn, which no real
 *  terminal multiplexer does. Only closing the tab actually tears this down. */
function TerminalView({ id, visible }: { id: string; visible: boolean }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<XTerm | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    let disposed = false;
    let ptyId: string | null = null;
    let resizeObserver: ResizeObserver | null = null;
    let rafId = 0;

    void (async () => {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
      ]);
      if (disposed) return;

      const term = new Terminal({
        theme: xtermTheme(),
        fontFamily:
          'ui-monospace, "Cascadia Code", "SF Mono", Consolas, "JetBrains Mono", monospace',
        fontSize: 13,
        cursorBlink: true,
        allowProposedApi: true,
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.open(container);
      // xterm may treat Ctrl+` as a no-op key; explicitly let it bubble so
      // the app-level Ctrl+` panel toggle always gets first refusal, even
      // while the terminal has focus.
      term.attachCustomKeyEventHandler((e) => {
        if (e.type === "keydown" && e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey && e.key === "`") {
          return false;
        }
        return true;
      });
      termRef.current = term;
      fit.fit();

      try {
        // Clamp against the panel's opening height transition: the very
        // first fit() can land mid-animation and measure a near-zero
        // container. The ResizeObserver below corrects the real size a
        // moment later, but the initial spawn should never request a
        // degenerate 0x0 PTY.
        ptyId = await ptyClient.spawn(
          { rows: Math.max(1, term.rows), cols: Math.max(1, term.cols) },
          (bytes) => term.write(bytes),
        );
      } catch {
        term.writeln("\x1b[31mCouldn't start a shell session.\x1b[0m");
        return;
      }
      if (disposed) {
        await ptyClient.kill(ptyId).catch(() => {});
        return;
      }

      term.onData((data) => {
        if (ptyId) void ptyClient.write(ptyId, data).catch(() => {});
      });

      const applyResize = () => {
        fit.fit();
        if (ptyId) void ptyClient.resize(ptyId, term.cols, term.rows).catch(() => {});
      };
      resizeObserver = new ResizeObserver(() => {
        cancelAnimationFrame(rafId);
        rafId = requestAnimationFrame(applyResize);
      });
      resizeObserver.observe(container);
    })();

    return () => {
      disposed = true;
      cancelAnimationFrame(rafId);
      resizeObserver?.disconnect();
      termRef.current?.dispose();
      termRef.current = null;
      if (ptyId) void ptyClient.kill(ptyId).catch(() => {});
    };
    // Mount once per session id; visible toggles CSS only (see className below).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  // Re-apply the palette when this tab becomes visible, so a theme toggle
  // that happened while the panel was hidden/backgrounded still catches up
  // (simpler than a live MutationObserver on the document root).
  useEffect(() => {
    if (visible && termRef.current) {
      termRef.current.options.theme = xtermTheme();
      termRef.current.focus();
    }
  }, [visible]);

  return <div ref={containerRef} className={cn("h-full w-full p-1.5", !visible && "hidden")} />;
}

function useDragResize(height: number, setHeight: (h: number) => void) {
  return useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      const startY = e.clientY;
      const startHeight = height;
      const max = Math.round(window.innerHeight * 0.7);
      const onMove = (ev: MouseEvent) => {
        const next = startHeight - (ev.clientY - startY);
        setHeight(Math.min(max, Math.max(MIN_HEIGHT, next)));
      };
      const onUp = () => {
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [height, setHeight],
  );
}

/** VS Code-style docked terminal panel. Always mounted (App.tsx renders it
 *  unconditionally) so sessions keep running in the background when the
 *  panel is toggled closed — `open` only controls visible height. */
export function TerminalPanel({
  open,
  onSessionsChange,
}: {
  open: boolean;
  onSessionsChange?: (count: number) => void;
}) {
  const [height, setHeight] = useState(DEFAULT_HEIGHT);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const nextNum = useRef(1);
  const onDragStart = useDragResize(height, setHeight);

  const addSession = useCallback(() => {
    const id = `view-${nextNum.current}`;
    const label = String(nextNum.current);
    nextNum.current += 1;
    setSessions((prev) => [...prev, { id, label }]);
    setActiveId(id);
  }, []);

  const closeSession = useCallback(
    (id: string) => {
      setSessions((prev) => {
        const next = prev.filter((s) => s.id !== id);
        if (activeId === id) setActiveId(next[next.length - 1]?.id ?? null);
        return next;
      });
    },
    [activeId],
  );

  useEffect(() => {
    onSessionsChange?.(sessions.length);
  }, [sessions.length, onSessionsChange]);

  // Open with at least one session.
  useEffect(() => {
    if (open && sessions.length === 0) addSession();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  if (!hasTauri()) {
    return open ? (
      <div
        style={{ height }}
        className="flex shrink-0 flex-col items-center justify-center border-t border-border bg-page text-sm text-muted"
      >
        The terminal is only available in the packaged app, not the browser dev preview.
      </div>
    ) : null;
  }

  return (
    <div
      style={{ height: open ? height : 0 }}
      className={cn(
        "flex shrink-0 flex-col overflow-hidden border-border bg-page transition-[height] duration-150",
        open && "border-t",
      )}
    >
      <div onMouseDown={onDragStart} className="h-1 shrink-0 cursor-row-resize hover:bg-primary/30" />
      <div className="flex shrink-0 items-center gap-1 border-b border-border bg-surface-2 px-1.5 py-1">
        <TerminalSquare className="ml-1 size-3.5 shrink-0 text-muted" />
        <div className="flex flex-1 items-center gap-0.5 overflow-x-auto">
          {sessions.map((s) => (
            <button
              key={s.id}
              onClick={() => setActiveId(s.id)}
              className={cn(
                "group flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-xs transition-colors",
                activeId === s.id
                  ? "bg-surface text-foreground shadow-sm"
                  : "text-secondary hover:bg-accent/60 hover:text-foreground",
              )}
            >
              {s.label}
              <span
                role="button"
                tabIndex={-1}
                onClick={(e) => {
                  e.stopPropagation();
                  closeSession(s.id);
                }}
                className="rounded p-0.5 opacity-0 hover:bg-critical/20 hover:text-critical group-hover:opacity-100"
                aria-label={`Close terminal ${s.label}`}
              >
                <X className="size-3" />
              </span>
            </button>
          ))}
        </div>
        <Button variant="ghost" size="icon" className="size-6" onClick={addSession} title="New terminal">
          <Plus className="size-3.5" />
        </Button>
      </div>
      <div className="relative flex-1 overflow-hidden">
        {sessions.map((s) => (
          <TerminalView key={s.id} id={s.id} visible={open && activeId === s.id} />
        ))}
      </div>
    </div>
  );
}

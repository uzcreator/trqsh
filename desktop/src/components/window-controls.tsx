import { useEffect, useState } from "react";
import { host } from "@/lib/agent";
import { cn } from "@/lib/utils";

// Native window buttons for a frameless window. On macOS the OS draws its own
// traffic lights (MacTitleBarHiddenInset in main.go), so we render nothing and
// let the titlebar reserve space instead. On Windows/Linux we draw min / max /
// close ourselves and drive them through the Go WindowService.

function Glyph({ kind, maximized }: { kind: "min" | "max" | "close"; maximized: boolean }) {
  const stroke = "currentColor";
  if (kind === "min") {
    return (
      <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden>
        <line x1="1.5" y1="5" x2="8.5" y2="5" stroke={stroke} strokeWidth="1" />
      </svg>
    );
  }
  if (kind === "close") {
    return (
      <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden>
        <line x1="1.5" y1="1.5" x2="8.5" y2="8.5" stroke={stroke} strokeWidth="1" />
        <line x1="8.5" y1="1.5" x2="1.5" y2="8.5" stroke={stroke} strokeWidth="1" />
      </svg>
    );
  }
  // maximize / restore
  return maximized ? (
    <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden>
      <rect x="1.5" y="2.8" width="5.7" height="5.7" fill="none" stroke={stroke} strokeWidth="1" />
      <path d="M3.3 2.8V1.5h5.2v5.2H7.2" fill="none" stroke={stroke} strokeWidth="1" />
    </svg>
  ) : (
    <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden>
      <rect x="1.5" y="1.5" width="7" height="7" fill="none" stroke={stroke} strokeWidth="1" />
    </svg>
  );
}

/** Windows/Linux window buttons. `onClose` lets the app honor the
 *  minimize-to-tray preference (hide vs. quit). Renders nothing on macOS. */
export function WindowControls({
  os,
  onClose,
}: {
  os: string;
  onClose: () => void;
}) {
  const [maximized, setMaximized] = useState(false);

  useEffect(() => {
    host.isMaximized().then(setMaximized).catch(() => {});
  }, []);

  if (os === "darwin") return null;

  const btn =
    "flex h-8 w-11 items-center justify-center text-secondary transition-colors hover:bg-accent hover:text-foreground";

  // Only elements tagged data-tauri-drag-region drag the window, so these
  // buttons are click targets by default — no explicit no-drag needed.
  return (
    <div className="flex items-center">
      <button className={btn} title="Minimize" onClick={() => host.minimize()}>
        <Glyph kind="min" maximized={maximized} />
      </button>
      <button
        className={btn}
        title={maximized ? "Restore" : "Maximize"}
        onClick={async () => {
          await host.toggleMaximize();
          setMaximized((m) => !m);
        }}
      >
        <Glyph kind="max" maximized={maximized} />
      </button>
      <button
        className={cn(btn, "hover:bg-critical hover:text-white")}
        title="Close"
        onClick={onClose}
      >
        <Glyph kind="close" maximized={maximized} />
      </button>
    </div>
  );
}

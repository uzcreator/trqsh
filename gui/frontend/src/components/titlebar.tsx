import { Moon, Sun } from "lucide-react";
import type { Status } from "@/lib/types";
import { applyTheme, storedTheme, type Theme } from "@/lib/theme";
import { StatusDot } from "./ui/status-dot";
import { Button } from "./ui/button";
import { useState } from "react";

function RiftMark() {
  return (
    <div className="flex items-center gap-2">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden>
        <path
          d="M12 2 3 12l9 10 9-10-9-10Z"
          className="fill-primary"
          opacity="0.15"
        />
        <path
          d="M12 2v20M3 12l9-4 9 4"
          className="stroke-primary"
          strokeWidth="1.75"
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      </svg>
      <span className="text-sm font-semibold tracking-tight">Rift</span>
    </div>
  );
}

/** Frameless-window titlebar: brand, connection pill, theme toggle. The drag
 *  region works when the Go side runs a frameless window and is inert otherwise. */
export function Titlebar({ status }: { status: Status }) {
  const [theme, setTheme] = useState<Theme>(storedTheme());

  const cycle = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    applyTheme(next);
  };

  const conn = status.connected
    ? { dot: "online" as const, text: status.edge || "Connected" }
    : { dot: "offline" as const, text: "Disconnected" };

  return (
    <header
      className="flex h-11 shrink-0 items-center justify-between border-b border-border bg-surface px-3"
      style={{ ["--wails-draggable" as any]: "drag" }}
    >
      <RiftMark />
      <div
        className="flex items-center gap-2"
        style={{ ["--wails-draggable" as any]: "no-drag" }}
      >
        <div className="flex items-center gap-1.5 rounded-full border border-border bg-page px-2.5 py-1">
          <StatusDot status={conn.dot} />
          <span className="text-xs text-secondary">{conn.text}</span>
        </div>
        <Button variant="ghost" size="icon" className="size-8" onClick={cycle} title="Toggle theme">
          {theme === "dark" ? <Sun /> : <Moon />}
        </Button>
      </div>
    </header>
  );
}

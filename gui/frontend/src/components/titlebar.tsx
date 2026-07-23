import { Moon, Search, Sun } from "lucide-react";
import type { Status } from "@/lib/types";
import type { Theme } from "@/lib/theme";
import { modKey } from "@/lib/hooks";
import { cn } from "@/lib/utils";
import { StatusDot } from "./ui/status-dot";
import { Button } from "./ui/button";
import { WindowControls } from "./window-controls";

function TrqshMark() {
  return (
    <div className="flex items-center gap-2">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden>
        <path d="M12 2 3 12l9 10 9-10-9-10Z" className="fill-primary" opacity="0.15" />
        <path
          d="M12 2v20M3 12l9-4 9 4"
          className="stroke-primary"
          strokeWidth="1.75"
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      </svg>
      <span className="text-sm font-semibold tracking-tight">trqsh</span>
    </div>
  );
}

/** Frameless-window titlebar: brand + connection pill (left, draggable), and a
 *  command trigger, theme toggle, and native window buttons (right, no-drag).
 *  On macOS the OS draws its own traffic lights, so we pad the brand clear of
 *  them and WindowControls renders nothing. */
export function Titlebar({
  status,
  os,
  theme,
  onToggleTheme,
  onOpenPalette,
  onClose,
}: {
  status: Status;
  os: string;
  theme: Theme;
  onToggleTheme: () => void;
  onOpenPalette: () => void;
  onClose: () => void;
}) {
  const conn = status.connected
    ? { dot: "online" as const, text: status.edge || "Connected" }
    : { dot: "offline" as const, text: "Disconnected" };
  const isMac = os === "darwin";

  return (
    <header className="drag flex h-11 shrink-0 items-center justify-between border-b border-border bg-surface">
      <div className={cn("flex items-center gap-2.5 pl-3", isMac && "pl-[76px]")}>
        <TrqshMark />
        <div className="flex items-center gap-1.5 rounded-full border border-border bg-page px-2.5 py-1">
          <StatusDot status={conn.dot} />
          <span className="max-w-40 truncate text-xs text-secondary">{conn.text}</span>
        </div>
      </div>

      <div className="no-drag flex items-center">
        <button
          onClick={onOpenPalette}
          title={`Command palette (${modKey} K)`}
          className="mr-1 flex items-center gap-2 rounded-md border border-border bg-page px-2 py-1 text-xs text-muted transition-colors hover:text-foreground"
        >
          <Search className="size-3.5" />
          <span className="hidden sm:inline">Search</span>
          <kbd className="hidden rounded bg-border/60 px-1 text-[10px] sm:inline">{modKey} K</kbd>
        </button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          onClick={onToggleTheme}
          title="Toggle theme"
        >
          {theme === "dark" ? <Sun /> : <Moon />}
        </Button>
        <div className={cn(isMac ? "pr-2" : "ml-1")}>
          <WindowControls os={os} onClose={onClose} />
        </div>
      </div>
    </header>
  );
}

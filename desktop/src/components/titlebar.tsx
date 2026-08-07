import { Moon, Search, Sun } from "lucide-react";
import type { Theme } from "@/lib/theme";
import { modKey } from "@/lib/hooks";
import { cn } from "@/lib/utils";
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

/** Frameless-window titlebar: brand (left, draggable), and a command trigger,
 *  theme toggle, and native window buttons (right, no-drag). Connection state
 *  now lives solely in the bottom status bar (see status-bar.tsx) — showing it
 *  here too was a redundant second "vitals" indicator. On macOS the OS draws
 *  its own traffic lights, so we pad the brand clear of them and
 *  WindowControls renders nothing. */
export function Titlebar({
  os,
  theme,
  onToggleTheme,
  onOpenPalette,
  onClose,
}: {
  os: string;
  theme: Theme;
  onToggleTheme: () => void;
  onOpenPalette: () => void;
  onClose: () => void;
}) {
  const isMac = os === "darwin";

  return (
    <header
      data-tauri-drag-region
      className="flex h-11 shrink-0 items-center justify-between border-b border-border bg-surface"
    >
      <div
        data-tauri-drag-region
        className={cn("flex items-center gap-2.5 pl-3", isMac && "pl-[76px]")}
      >
        <TrqshMark />
      </div>

      <div className="flex items-center">
        <button
          onClick={onOpenPalette}
          title={`Command palette (${modKey} K)`}
          className="mr-1 flex items-center gap-2 rounded-md border border-border bg-page px-2 py-1 text-xs text-muted transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-page"
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

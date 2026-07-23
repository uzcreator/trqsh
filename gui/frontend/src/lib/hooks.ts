// Small dependency-free React hooks shared across the app: responsive width,
// a ticking clock for live durations/rates, and a global hotkey matcher so the
// GUI is keyboard-driven like a real desktop app.

import { useEffect, useState } from "react";

/** Current window inner width, updated on resize (rAF-throttled). Drives the
 *  responsive breakpoints (collapsible sidebar, adaptive inspector split). */
export function useWindowWidth(): number {
  const [w, setW] = useState(() =>
    typeof window === "undefined" ? 1024 : window.innerWidth,
  );
  useEffect(() => {
    let raf = 0;
    const onResize = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => setW(window.innerWidth));
    };
    window.addEventListener("resize", onResize);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", onResize);
    };
  }, []);
  return w;
}

/** A Date that advances every `ms`. Used for tunnel uptime and request-rate
 *  windows. Pass a coarse interval (1000ms) to keep re-renders cheap. */
export function useNow(ms = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(id);
  }, [ms]);
  return now;
}

export interface Hotkey {
  /** e.g. "mod+k" (mod = ⌘ on mac, Ctrl elsewhere), "mod+shift+n", "1". */
  combo: string;
  handler: (e: KeyboardEvent) => void;
  /** Fire even while focus is in an input/textarea (default false). */
  allowInInput?: boolean;
}

const isMac =
  typeof navigator !== "undefined" && /mac/i.test(navigator.userAgent);

function matches(e: KeyboardEvent, combo: string): boolean {
  const parts = combo.toLowerCase().split("+");
  const key = parts[parts.length - 1];
  const want = {
    mod: parts.includes("mod"),
    shift: parts.includes("shift"),
    alt: parts.includes("alt"),
  };
  const mod = isMac ? e.metaKey : e.ctrlKey;
  // The "other" primary modifier must be off so ⌘K doesn't also fire Ctrl+K.
  const otherPrimary = isMac ? e.ctrlKey : e.metaKey;
  if (want.mod !== mod) return false;
  if (want.mod && otherPrimary) return false;
  if (want.shift !== e.shiftKey) return false;
  if (want.alt !== e.altKey) return false;
  return e.key.toLowerCase() === key;
}

function inEditable(el: EventTarget | null): boolean {
  const n = el as HTMLElement | null;
  if (!n) return false;
  const tag = n.tagName;
  return (
    tag === "INPUT" ||
    tag === "TEXTAREA" ||
    tag === "SELECT" ||
    n.isContentEditable
  );
}

/** Register global keyboard shortcuts for the lifetime of the component. */
export function useHotkeys(keys: Hotkey[]) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const editing = inEditable(e.target);
      for (const k of keys) {
        if (!k.allowInInput && editing) continue;
        if (matches(e, k.combo)) {
          e.preventDefault();
          k.handler(e);
          return;
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [keys]);
}

/** Platform-aware label for the ⌘/Ctrl modifier, for shortcut hints. */
export const modKey = isMac ? "⌘" : "Ctrl";

// Theme application. The desktop app has no server to persist a cookie (unlike
// web/dashboard), so the choice lives in AppSettings (Go side) and is mirrored
// to localStorage for an instant, flash-free apply on the next launch.

export type Theme = "system" | "light" | "dark";

const KEY = "rift.theme";

function systemPrefersDark(): boolean {
  return (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-color-scheme: dark)").matches
  );
}

/** Resolve + apply a theme to <html>, toggling the `.dark` class. */
export function applyTheme(theme: Theme) {
  const dark = theme === "dark" || (theme === "system" && systemPrefersDark());
  const root = document.documentElement;
  root.classList.toggle("dark", dark);
  root.style.colorScheme = dark ? "dark" : "light";
  try {
    localStorage.setItem(KEY, theme);
  } catch {
    /* storage unavailable; non-fatal */
  }
}

/** Read the last theme (for the no-flash boot in main.tsx). */
export function storedTheme(): Theme {
  try {
    const t = localStorage.getItem(KEY);
    if (t === "light" || t === "dark" || t === "system") return t;
  } catch {
    /* ignore */
  }
  return "system";
}

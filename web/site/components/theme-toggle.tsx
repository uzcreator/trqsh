"use client";

import * as React from "react";
import { Moon, Sun } from "lucide-react";

// Client-only theme toggle backed by localStorage (not a cookie), so every
// marketing/docs page stays statically renderable — no per-request work. The
// pre-paint script in app/layout.tsx applies the stored value before first paint
// to avoid a flash.
export function ThemeToggle({ className }: { className?: string }) {
  const [dark, setDark] = React.useState(false);

  React.useEffect(() => {
    setDark(document.documentElement.classList.contains("dark"));
  }, []);

  function toggle() {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    try {
      localStorage.setItem("theme", next ? "dark" : "light");
    } catch {
      /* storage unavailable */
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label="Toggle color theme"
      className={
        "inline-flex h-9 w-9 items-center justify-center rounded-md border border-border-strong bg-surface text-secondary transition-colors hover:bg-accent " +
        (className ?? "")
      }
    >
      <Sun className="hidden h-4 w-4 dark:block" />
      <Moon className="block h-4 w-4 dark:hidden" />
    </button>
  );
}

"use client";

import * as React from "react";
import { OS_DOWNLOADS, OS_ORDER, type OSId } from "@/lib/downloads";
import { CodeBlock } from "./code-block";
import { cn } from "@/lib/utils";

function detectOS(): OSId {
  if (typeof navigator === "undefined") return "macos";
  const s = `${navigator.userAgent} ${navigator.platform}`.toLowerCase();
  if (s.includes("win")) return "windows";
  if (s.includes("linux") || s.includes("android")) return "linux";
  return "macos";
}

// Compact per-OS install one-liner (auto-detects the visitor's OS). Used in the
// landing quickstart; the full matrix lives on /download.
export function InstallTabs() {
  const [active, setActive] = React.useState<OSId>("macos");
  React.useEffect(() => setActive(detectOS()), []);

  const primary = OS_DOWNLOADS[active].cli[0];

  return (
    <div>
      <div className="mb-2 inline-flex gap-1 rounded-md border border-border-strong bg-surface p-0.5">
        {OS_ORDER.map((id) => (
          <button
            key={id}
            type="button"
            onClick={() => setActive(id)}
            className={cn(
              "rounded px-3 py-1 text-xs font-medium transition-colors",
              active === id ? "bg-accent text-primary" : "text-secondary hover:text-foreground"
            )}
          >
            {OS_DOWNLOADS[id].name}
          </button>
        ))}
      </div>
      <CodeBlock code={primary.command} prompt />
    </div>
  );
}

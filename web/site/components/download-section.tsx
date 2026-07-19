"use client";

import * as React from "react";
import { Apple, Download, Monitor, Terminal as TerminalIcon } from "lucide-react";
import { OS_DOWNLOADS, OS_ORDER, checksumsUrl, releasesUrl, type OSId } from "@/lib/downloads";
import { site } from "@/lib/site";
import { CodeBlock } from "./code-block";
import { buttonVariants } from "./ui/button";
import { Badge } from "./ui/badge";
import { cn } from "@/lib/utils";

function detectOS(): OSId {
  if (typeof navigator === "undefined") return "macos";
  const s = `${navigator.userAgent} ${navigator.platform}`.toLowerCase();
  if (s.includes("win")) return "windows";
  if (s.includes("linux") || s.includes("android")) return "linux";
  return "macos";
}

const OS_ICON: Record<OSId, React.ComponentType<{ className?: string }>> = {
  macos: Apple,
  windows: Monitor,
  linux: TerminalIcon,
};

export function DownloadSection() {
  const [active, setActive] = React.useState<OSId>("macos");

  React.useEffect(() => {
    setActive(detectOS());
  }, []);

  const os = OS_DOWNLOADS[active];

  return (
    <div>
      <div className="mb-6 inline-flex flex-wrap gap-1 rounded-md border border-border-strong bg-surface p-0.5">
        {OS_ORDER.map((id) => {
          const Icon = OS_ICON[id];
          return (
            <button
              key={id}
              type="button"
              onClick={() => setActive(id)}
              className={cn(
                "flex items-center gap-2 rounded px-4 py-1.5 text-sm font-medium transition-colors",
                active === id ? "bg-accent text-primary" : "text-secondary hover:text-foreground"
              )}
            >
              <Icon className="h-4 w-4" />
              {OS_DOWNLOADS[id].name}
            </button>
          );
        })}
      </div>

      <div className="grid gap-5 lg:grid-cols-2">
        {/* Desktop app */}
        <div className="rounded-lg border border-border bg-surface p-6 shadow-sm">
          <div className="mb-1 flex items-center gap-2">
            <h3 className="text-base font-semibold text-foreground">Desktop app</h3>
            <Badge variant="good">Recommended</Badge>
          </div>
          <p className="mb-4 text-sm text-secondary">
            One-click tunnels, a built-in inspector, and a system tray. Signed &amp; notarized.
          </p>
          <div className="flex flex-col gap-2">
            {os.desktop.map((a) => (
              <a key={a.href} href={a.href} className={cn(buttonVariants(), "w-full justify-center")}>
                <Download className="h-4 w-4" />
                {a.label}
              </a>
            ))}
          </div>
          <p className="mt-3 text-xs text-muted">
            Version {site.version} ·{" "}
            <a href={checksumsUrl} className="underline hover:text-foreground">
              checksums.txt
            </a>
          </p>
        </div>

        {/* CLI */}
        <div className="rounded-lg border border-border bg-surface p-6 shadow-sm">
          <h3 className="mb-1 text-base font-semibold text-foreground">Command line</h3>
          <p className="mb-4 text-sm text-secondary">Prefer the terminal? Install the open-source CLI:</p>
          <div className="flex flex-col gap-3">
            {os.cli.map((snip) => (
              <div key={snip.label}>
                <div className="mb-1 text-xs font-medium text-muted">{snip.label}</div>
                <CodeBlock code={snip.command} prompt />
              </div>
            ))}
          </div>
          <div className="mt-4">
            <div className="mb-1 text-xs font-medium text-muted">Or download the binary</div>
            <div className="flex flex-wrap gap-2">
              {os.archives.map((a) => (
                <a
                  key={a.href}
                  href={a.href}
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }))}
                >
                  {a.arch ?? "download"}
                </a>
              ))}
            </div>
          </div>
        </div>
      </div>

      <p className="mt-5 text-sm text-muted">
        Looking for `.deb`, `.rpm`, older versions, or the edge server?{" "}
        <a href={releasesUrl} className="text-primary hover:underline">
          Browse all releases →
        </a>
      </p>
    </div>
  );
}

"use client";

import * as React from "react";
import { Apple, Download, Monitor, Star, Terminal as TerminalIcon } from "lucide-react";
import {
  OS_DOWNLOADS,
  OS_ORDER,
  checksumsUrl,
  releasesUrl,
  type OSId,
} from "@/lib/downloads";
import { detectOS } from "@/lib/detect";
import { site } from "@/lib/site";
import { CodeBlock } from "./code-block";
import { buttonVariants } from "./ui/button";
import { Badge } from "./ui/badge";
import { cn } from "@/lib/utils";

const OS_ICON: Record<OSId, React.ComponentType<{ className?: string }>> = {
  macos: Apple,
  windows: Monitor,
  linux: TerminalIcon,
};

// Full download matrix — every OS, its desktop build, package-manager commands,
// and raw archives shown at once. The visitor's detected OS floats to the front
// and gets a highlighted ring so they see their platform first.
export function DownloadSection() {
  const [detected, setDetected] = React.useState<OSId | null>(null);
  React.useEffect(() => setDetected(detectOS()), []);

  const order = React.useMemo(() => {
    if (!detected) return OS_ORDER;
    return [detected, ...OS_ORDER.filter((o) => o !== detected)];
  }, [detected]);

  return (
    <div>
      <div className="grid gap-5 lg:grid-cols-3">
        {order.map((id) => {
          const os = OS_DOWNLOADS[id];
          const Icon = OS_ICON[id];
          const isYou = detected === id;
          return (
            <div
              key={id}
              className={cn(
                "border-gradient flex flex-col rounded-2xl border bg-surface p-6 shadow-sm transition-shadow",
                isYou ? "border-brand/40 glow-brand" : "border-border"
              )}
            >
              <div className="mb-4 flex items-center gap-3">
                <span className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-accent text-brand">
                  <Icon className="h-5 w-5" />
                </span>
                <div>
                  <div className="flex items-center gap-2">
                    <h3 className="text-base font-semibold text-foreground">{os.name}</h3>
                    {isYou && <Badge variant="good">Your OS</Badge>}
                  </div>
                  <p className="text-xs text-muted">{os.tagline}</p>
                </div>
              </div>

              {/* Desktop app */}
              <div className="flex flex-col gap-2">
                {os.desktop.map((a) => (
                  <a
                    key={a.href}
                    href={a.href}
                    className={cn(buttonVariants({ size: "sm" }), "w-full justify-center")}
                  >
                    <Download className="h-4 w-4" /> {a.label}
                    {a.kind && <span className="opacity-70">· {a.kind}</span>}
                  </a>
                ))}
              </div>

              {/* Package managers */}
              <div className="mt-5">
                <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
                  Package managers
                </p>
                <div className="flex flex-col gap-2.5">
                  {os.cli.map((snip) => (
                    <div key={snip.label}>
                      <div className="mb-1 flex items-center gap-1.5 text-xs font-medium text-secondary">
                        {snip.recommended && <Star className="h-3 w-3 fill-brand text-brand" />}
                        {snip.label}
                      </div>
                      <CodeBlock code={snip.command} prompt />
                    </div>
                  ))}
                </div>
              </div>

              {/* Raw archives */}
              <div className="mt-5 border-t border-border pt-4">
                <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
                  Direct binaries
                </p>
                <div className="flex flex-wrap gap-2">
                  {os.archives.map((a) => (
                    <a
                      key={a.href}
                      href={a.href}
                      className={cn(buttonVariants({ variant: "outline", size: "sm" }), "text-xs")}
                    >
                      {a.label}
                      {a.kind && <span className="text-muted">· {a.kind}</span>}
                    </a>
                  ))}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      <div className="mt-6 flex flex-wrap items-center justify-between gap-3 text-sm text-muted">
        <p>
          Version {site.version} ·{" "}
          <a href={checksumsUrl} className="text-brand hover:underline">
            checksums.txt
          </a>{" "}
          · every build is signed
        </p>
        <a href={releasesUrl} className="text-brand hover:underline">
          Browse all releases →
        </a>
      </div>
    </div>
  );
}

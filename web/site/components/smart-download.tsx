"use client";

import * as React from "react";
import { Apple, ArrowRight, Cpu, Download, Monitor, Terminal as TerminalIcon } from "lucide-react";
import Link from "next/link";
import {
  ARCH_LABEL,
  OS_DOWNLOADS,
  OS_LABEL,
  OS_ORDER,
  primaryCli,
  primaryDownload,
  type Arch,
  type OSId,
} from "@/lib/downloads";
import { detectArch, detectOS } from "@/lib/detect";
import { site } from "@/lib/site";
import { CodeBlock } from "./code-block";
import { buttonVariants } from "./ui/button";
import { BorderBeam } from "./lightswind/border-beam";
import { cn } from "@/lib/utils";

const OS_ICON: Record<OSId, React.ComponentType<{ className?: string }>> = {
  macos: Apple,
  windows: Monitor,
  linux: TerminalIcon,
};

// OS/arch-aware download. On mount it detects the visitor's platform and offers
// the exact desktop build + the idiomatic CLI one-liner, while still letting them
// switch OS. `variant="hero"` is the compact landing form; `variant="card"` is the
// framed panel used on /download and the closing CTA.
export function SmartDownload({
  variant = "card",
  className,
}: {
  variant?: "hero" | "card";
  className?: string;
}) {
  const [os, setOs] = React.useState<OSId>("macos");
  const [arch, setArch] = React.useState<Arch | null>(null);
  const [auto, setAuto] = React.useState(true);

  React.useEffect(() => {
    const detected = detectOS();
    setOs(detected);
    let alive = true;
    detectArch(detected).then((a) => alive && setArch(a));
    return () => {
      alive = false;
    };
  }, []);

  const primary = primaryDownload(os, arch);
  const isApp = OS_DOWNLOADS[os].desktop.length > 0;
  const cli = primaryCli(os);

  const switcher = (
    <div className="inline-flex gap-1 rounded-lg border border-border-strong bg-surface/70 p-1">
      {OS_ORDER.map((id) => {
        const OsIcon = OS_ICON[id];
        return (
          <button
            key={id}
            type="button"
            onClick={() => {
              setOs(id);
              setAuto(false);
              detectArch(id).then(setArch);
            }}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
              os === id ? "bg-accent text-brand" : "text-secondary hover:text-foreground"
            )}
            aria-pressed={os === id}
          >
            <OsIcon className="h-3.5 w-3.5" />
            {OS_LABEL[id]}
          </button>
        );
      })}
    </div>
  );

  const detectedNote = (
    <span className="inline-flex items-center gap-1.5 text-xs text-muted">
      <Cpu className="h-3.5 w-3.5 text-brand" />
      {auto ? "Auto-detected" : "Selected"}: {OS_LABEL[os]}
      {arch ? ` · ${ARCH_LABEL[arch]}` : ""}
    </span>
  );

  if (variant === "hero") {
    return (
      <div className={cn("flex flex-col gap-3", className)}>
        <div className="flex flex-wrap items-center gap-3">
          <a
            href={primary.href}
            className={cn(
              buttonVariants({ size: "xl" }),
              "btn-shine glow-brand group relative overflow-hidden border-2 border-white/15"
            )}
          >
            <Download className="h-5 w-5" />
            <span className="flex flex-col items-start leading-tight">
              <span>Download for {OS_LABEL[os]}</span>
              {arch && (
                <span className="text-[0.7rem] font-normal opacity-80">{ARCH_LABEL[arch]}</span>
              )}
            </span>
            <BorderBeam size={32} duration={5} borderWidth={2} />
          </a>
          <Link
            href="/download"
            className={cn(buttonVariants({ variant: "outline", size: "xl" }))}
          >
            All platforms <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "glass relative overflow-hidden rounded-2xl p-6 sm:p-7",
        className
      )}
    >
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        {detectedNote}
        {switcher}
      </div>

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        <div className="flex flex-col">
          <span className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
            {isApp ? "Desktop app · recommended" : "Direct download · recommended"}
          </span>
          <a
            href={primary.href}
            className={cn(buttonVariants({ size: "lg" }), "btn-shine glow-brand w-full justify-center")}
          >
            <Download className="h-4 w-4" />
            Download for {OS_LABEL[os]}
          </a>
          <span className="mt-2 text-xs text-muted">
            {isApp
              ? `Desktop app · v${site.version} · unsigned preview`
              : `${primary.kind ? `${primary.kind} · ` : ""}v${site.version} · checksummed`}
          </span>
        </div>

        <div className="flex flex-col">
          <span className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
            Or install the CLI · {cli.label}
          </span>
          <CodeBlock code={cli.command} prompt />
          <Link href="/download" className="mt-2 inline-flex items-center gap-1 text-xs text-brand hover:underline">
            More package managers &amp; checksums <ArrowRight className="h-3 w-3" />
          </Link>
        </div>
      </div>
    </div>
  );
}

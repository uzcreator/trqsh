"use client";

import * as React from "react";
import { Apple, ArrowRight, Cpu, Download, Monitor, Terminal as TerminalIcon } from "lucide-react";
import Link from "next/link";
import {
  ARCH_LABEL,
  OS_DOWNLOADS,
  OS_LABEL,
  OS_ORDER,
  desktopFor,
  primaryCli,
  type Arch,
  type OSId,
} from "@/lib/downloads";
import { detectArch, detectOS } from "@/lib/detect";
import { site } from "@/lib/site";
import { CodeBlock } from "./code-block";
import { buttonVariants } from "./ui/button";
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

  const desktop = desktopFor(os, arch);
  const cli = primaryCli(os);
  const Icon = OS_ICON[os];

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
            href={desktop.href}
            className={cn(buttonVariants({ size: "xl" }), "btn-shine glow-brand group")}
          >
            <Download className="h-5 w-5" />
            <span className="flex flex-col items-start leading-tight">
              <span>Download for {OS_LABEL[os]}</span>
              {arch && (
                <span className="text-[0.7rem] font-normal opacity-80">{ARCH_LABEL[arch]}</span>
              )}
            </span>
          </a>
          <Link
            href="/download"
            className={cn(buttonVariants({ variant: "outline", size: "xl" }))}
          >
            All platforms <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
        <div className="flex items-center gap-3">
          <Icon className="h-3.5 w-3.5 text-muted" />
          {detectedNote}
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

      <div className="grid gap-5 sm:grid-cols-2">
        <div className="flex flex-col">
          <span className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
            Desktop app · recommended
          </span>
          <a
            href={desktop.href}
            className={cn(buttonVariants({ size: "lg" }), "btn-shine glow-brand w-full justify-center")}
          >
            <Download className="h-4 w-4" />
            {desktop.label}
          </a>
          <span className="mt-2 text-xs text-muted">
            {desktop.kind ? `${desktop.kind} · ` : ""}v{site.version} · signed &amp; notarized
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

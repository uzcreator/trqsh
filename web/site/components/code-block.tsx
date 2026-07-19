"use client";

import * as React from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

// A copyable code/command block used across the marketing pages and download
// snippets. Kept dependency-free (no syntax-highlight runtime) for speed; styling
// comes from the brand tokens.
export function CodeBlock({
  code,
  label,
  className,
  prompt = false,
}: {
  code: string;
  label?: string;
  className?: string;
  /** Render a leading `$` prompt on each line (shell commands). */
  prompt?: boolean;
}) {
  const [copied, setCopied] = React.useState(false);
  const lines = code.replace(/\n$/, "").split("\n");

  return (
    <div
      className={cn(
        "group relative overflow-hidden rounded-lg border border-border bg-surface text-left shadow-sm",
        className
      )}
    >
      {label && (
        <div className="flex items-center justify-between border-b border-border px-4 py-2">
          <span className="text-xs font-medium text-muted">{label}</span>
        </div>
      )}
      <button
        type="button"
        aria-label="Copy code"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(code);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          } catch {
            /* clipboard unavailable */
          }
        }}
        className="absolute right-2 top-2 z-10 inline-flex h-7 w-7 items-center justify-center rounded-md border border-border-strong bg-surface text-secondary opacity-0 transition-opacity hover:bg-accent focus-visible:opacity-100 group-hover:opacity-100"
      >
        {copied ? <Check className="h-3.5 w-3.5 text-good" /> : <Copy className="h-3.5 w-3.5" />}
      </button>
      <pre className="overflow-x-auto px-4 py-3.5 font-mono text-[0.82rem] leading-relaxed">
        <code>
          {lines.map((line, i) => (
            <span key={i} className="block">
              {prompt && line.length > 0 && <span className="select-none text-muted">$ </span>}
              {line || " "}
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

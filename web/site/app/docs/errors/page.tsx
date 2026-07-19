import type { Metadata } from "next";
import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { ERRORS, errorAnchor } from "@/lib/errors";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const metadata: Metadata = {
  title: "Error reference",
  description:
    "Every Rift error code, what it means, and how to fix it. The CLI, desktop app, and dashboard deep-link here.",
};

export default function ErrorsPage() {
  return (
    <div className="py-10">
      <div className="mb-8">
        <h1 className="text-3xl font-semibold tracking-tight text-foreground">Error reference</h1>
        <p className="mt-3 max-w-2xl text-secondary">
          When Rift declines a request, it returns one of these stable codes. The CLI, desktop app,
          and dashboard link straight to the matching entry below — so you always know what happened
          and what to do next.
        </p>
      </div>

      <div className="mb-8 flex flex-wrap gap-2">
        {ERRORS.map((e) => (
          <a
            key={e.code}
            href={`#${errorAnchor(e.code)}`}
            className="rounded-md border border-border bg-surface px-2.5 py-1 font-mono text-xs text-secondary transition-colors hover:border-border-strong hover:text-foreground"
          >
            {e.code}
          </a>
        ))}
      </div>

      <div className="flex flex-col gap-4">
        {ERRORS.map((e) => (
          <section
            key={e.code}
            id={errorAnchor(e.code)}
            className="scroll-mt-24 rounded-lg border border-border bg-surface p-6 shadow-sm"
          >
            <div className="flex flex-wrap items-center gap-3">
              <code className="rounded-md bg-accent px-2 py-1 font-mono text-sm font-medium text-primary">
                {e.code}
              </code>
              <h2 className="text-base font-semibold text-foreground">{e.title}</h2>
              {e.upgrade && <Badge variant="warning">Plan limit</Badge>}
            </div>
            <p className="mt-3 text-sm text-secondary">{e.detail}</p>
            <p className="mt-2 text-sm text-foreground">
              <span className="font-medium">Fix:</span> {e.fix}
            </p>
            {e.upgrade && (
              <Link href="/pricing" className={cn(buttonVariants({ variant: "subtle", size: "sm" }), "mt-4")}>
                Compare plans <ArrowUpRight className="h-3.5 w-3.5" />
              </Link>
            )}
          </section>
        ))}
      </div>
    </div>
  );
}

import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, BookOpen, Rocket, Terminal, Globe, KeyRound } from "lucide-react";
import { docsNav } from "@/lib/docs";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const metadata: Metadata = {
  title: "Documentation",
  description: "Guides, references, and a 60-second quickstart for trqsh.",
};

const HIGHLIGHTS = [
  { icon: Rocket, title: "Quickstart", body: "From install to a live URL in under a minute.", href: "/docs/quickstart" },
  { icon: Terminal, title: "HTTP tunnels", body: "Expose web apps, APIs, and webhooks.", href: "/docs/http-tunnels" },
  { icon: Globe, title: "Custom domains", body: "Bring your own domain with guided DNS.", href: "/docs/custom-domains" },
  { icon: KeyRound, title: "API reference", body: "The Control API, generated from OpenAPI.", href: "/docs/api" },
];

export default function DocsIndexPage() {
  return (
    <div className="py-10">
      <div className="mb-10">
        <div className="mb-3 inline-flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground">
          <BookOpen className="h-6 w-6" />
        </div>
        <h1 className="text-3xl font-semibold tracking-tight text-foreground">Documentation</h1>
        <p className="mt-3 max-w-2xl text-secondary">
          Everything you need to expose localhost with trqsh — from your first tunnel to custom
          domains, the request inspector, configuration, and the full API.
        </p>
        <div className="mt-6 flex flex-wrap gap-3">
          <Link href="/docs/quickstart" className={cn(buttonVariants())}>
            Start the quickstart <ArrowRight className="h-4 w-4" />
          </Link>
          <Link href="/docs/api" className={cn(buttonVariants({ variant: "outline" }))}>
            API reference
          </Link>
        </div>
      </div>

      <div className="mb-14 grid gap-4 sm:grid-cols-2">
        {HIGHLIGHTS.map((h) => (
          <Link
            key={h.href}
            href={h.href}
            className="card-hover group flex items-start gap-4 rounded-lg border border-border bg-surface p-5 shadow-sm"
          >
            <div className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-accent text-primary">
              <h.icon className="h-5 w-5" />
            </div>
            <div>
              <h3 className="flex items-center gap-1 text-base font-semibold text-foreground">
                {h.title}
                <ArrowRight className="h-4 w-4 -translate-x-1 opacity-0 transition-all group-hover:translate-x-0 group-hover:opacity-100" />
              </h3>
              <p className="mt-1 text-sm text-secondary">{h.body}</p>
            </div>
          </Link>
        ))}
      </div>

      <div className="flex flex-col gap-8">
        {docsNav.map((cat) => (
          <div key={cat.title}>
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">{cat.title}</h2>
            <ul className="grid gap-2 sm:grid-cols-2">
              {cat.items.map((item) => (
                <li key={item.slug}>
                  <Link
                    href={`/docs/${item.slug}`}
                    className="block rounded-md border border-border bg-surface px-4 py-3 transition-colors hover:border-border-strong"
                  >
                    <span className="text-sm font-medium text-foreground">{item.title}</span>
                    {item.description && (
                      <span className="mt-0.5 block text-xs text-muted">{item.description}</span>
                    )}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </div>
  );
}

"use client";

import * as React from "react";
import Link from "next/link";
import { Github, Menu, X } from "lucide-react";
import { Logo } from "./logo";
import { ThemeToggle } from "./theme-toggle";
import { buttonVariants } from "./ui/button";
import { primaryNav, signupUrl, loginUrl, site } from "@/lib/site";
import { cn } from "@/lib/utils";

export function SiteHeader() {
  const [open, setOpen] = React.useState(false);

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-page/80 backdrop-blur supports-[backdrop-filter]:bg-page/70">
      <div className="mx-auto flex h-16 max-w-content items-center justify-between gap-4 px-4 sm:px-6">
        <div className="flex items-center gap-8">
          <Link href="/" aria-label="Rift home" className="shrink-0">
            <Logo />
          </Link>
          <nav className="hidden items-center gap-1 md:flex">
            {primaryNav.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="rounded-md px-3 py-2 text-sm font-medium text-secondary transition-colors hover:bg-accent hover:text-foreground"
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </div>

        <div className="hidden items-center gap-2 md:flex">
          <a
            href={site.githubUrl}
            target="_blank"
            rel="noreferrer"
            aria-label="Rift on GitHub"
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border-strong bg-surface text-secondary transition-colors hover:bg-accent"
          >
            <Github className="h-4 w-4" />
          </a>
          <ThemeToggle />
          <a href={loginUrl} className={cn(buttonVariants({ variant: "ghost", size: "sm" }))}>
            Log in
          </a>
          <a href={signupUrl} className={cn(buttonVariants({ size: "sm" }))}>
            Start free
          </a>
        </div>

        <div className="flex items-center gap-2 md:hidden">
          <ThemeToggle />
          <button
            type="button"
            aria-label="Toggle menu"
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border-strong bg-surface text-secondary"
          >
            {open ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
          </button>
        </div>
      </div>

      {open && (
        <div className="border-t border-border bg-page md:hidden">
          <nav className="mx-auto flex max-w-content flex-col gap-1 px-4 py-3">
            {primaryNav.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                onClick={() => setOpen(false)}
                className="rounded-md px-3 py-2 text-sm font-medium text-secondary hover:bg-accent hover:text-foreground"
              >
                {item.label}
              </Link>
            ))}
            <div className="my-2 h-px bg-border" />
            <a href={loginUrl} className={cn(buttonVariants({ variant: "outline" }), "w-full")}>
              Log in
            </a>
            <a href={signupUrl} className={cn(buttonVariants(), "w-full")}>
              Start free
            </a>
          </nav>
        </div>
      )}
    </header>
  );
}

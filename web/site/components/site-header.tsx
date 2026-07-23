"use client";

import * as React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Github, Menu, X } from "lucide-react";
import { Logo } from "./logo";
import { ScrollProgress } from "./scroll-progress";
import { buttonVariants } from "./ui/button";
import { primaryNav, signupUrl, loginUrl, site } from "@/lib/site";
import { cn } from "@/lib/utils";

export function SiteHeader() {
  const [open, setOpen] = React.useState(false);
  const [scrolled, setScrolled] = React.useState(false);
  const pathname = usePathname();

  // Sliding highlight pill that tracks the hovered nav item.
  const [hl, setHl] = React.useState({ left: 0, width: 0, opacity: 0 });
  const onEnter = (e: React.MouseEvent<HTMLAnchorElement>) => {
    const t = e.currentTarget;
    setHl({ left: t.offsetLeft, width: t.offsetWidth, opacity: 1 });
  };

  React.useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname.startsWith(href);

  return (
    <header
      className={cn(
        "sticky top-0 z-40 animate-slide-down border-b transition-[background-color,border-color,box-shadow,backdrop-filter] duration-300",
        scrolled
          ? "border-border bg-page/80 shadow-[0_10px_30px_-20px_rgb(0_0_0/0.9)] backdrop-blur-xl supports-[backdrop-filter]:bg-page/60"
          : "border-transparent bg-transparent"
      )}
    >
      <div
        className={cn(
          "mx-auto flex max-w-content items-center justify-between gap-4 px-4 transition-[height] duration-300 sm:px-6",
          scrolled ? "h-14" : "h-16"
        )}
      >
        <div className="flex items-center gap-6">
          <Link href="/" aria-label="trqsh home" className="shrink-0">
            <Logo />
          </Link>
          <nav
            className="relative hidden items-center md:flex"
            onMouseLeave={() => setHl((h) => ({ ...h, opacity: 0 }))}
          >
            <span
              className="nav-highlight pointer-events-none absolute top-1/2 -z-0 h-8 -translate-y-1/2 rounded-md bg-accent"
              style={{ left: hl.left, width: hl.width, opacity: hl.opacity }}
              aria-hidden
            />
            {primaryNav.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                onMouseEnter={onEnter}
                className={cn(
                  "relative z-10 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive(item.href) ? "text-foreground" : "text-secondary hover:text-foreground"
                )}
              >
                {item.label}
                <span
                  className={cn(
                    "absolute inset-x-3 -bottom-0.5 h-0.5 rounded-full bg-brand transition-transform duration-300",
                    isActive(item.href) ? "scale-x-100" : "scale-x-0"
                  )}
                  aria-hidden
                />
              </Link>
            ))}
          </nav>
        </div>

        <div className="hidden items-center gap-2 md:flex">
          <a
            href={site.githubUrl}
            target="_blank"
            rel="noreferrer"
            aria-label="trqsh on GitHub"
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border-strong bg-surface/60 text-secondary transition-colors hover:border-brand/50 hover:bg-accent hover:text-foreground"
          >
            <Github className="h-4 w-4" />
          </a>
          <a href={loginUrl} className={cn(buttonVariants({ variant: "ghost", size: "sm" }))}>
            Log in
          </a>
          <a href={signupUrl} className={cn(buttonVariants({ size: "sm" }), "glow-brand")}>
            Start free
          </a>
        </div>

        <div className="flex items-center gap-2 md:hidden">
          <button
            type="button"
            aria-label="Toggle menu"
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border-strong bg-surface/60 text-secondary"
          >
            {open ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
          </button>
        </div>
      </div>

      <ScrollProgress />

      <div
        className={cn(
          "overflow-hidden border-border bg-page/95 backdrop-blur-xl transition-[max-height,opacity] duration-300 md:hidden",
          open ? "max-h-96 border-t opacity-100" : "max-h-0 opacity-0"
        )}
      >
        <nav className="mx-auto flex max-w-content flex-col gap-1 px-4 py-3">
          {primaryNav.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              onClick={() => setOpen(false)}
              className={cn(
                "rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-foreground",
                isActive(item.href) ? "bg-accent text-brand" : "text-secondary"
              )}
            >
              {item.label}
            </Link>
          ))}
          <div className="my-2 h-px bg-border" />
          <a href={loginUrl} className={cn(buttonVariants({ variant: "outline" }), "w-full")}>
            Log in
          </a>
          <a href={signupUrl} className={cn(buttonVariants(), "w-full glow-brand")}>
            Start free
          </a>
        </nav>
      </div>
    </header>
  );
}

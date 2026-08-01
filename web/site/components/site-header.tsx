"use client";

import * as React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Github, Menu, X } from "lucide-react";
import { Logo } from "./logo";
import { ScrollProgress } from "./scroll-progress";
import { buttonVariants } from "./ui/button";
import { primaryNav, signupUrl, site } from "@/lib/site";
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
    <header className="sticky top-0 z-40 animate-slide-down">
      <div
        className={cn(
          "flex items-center justify-between gap-4 transition-all duration-500 ease-out",
          scrolled
            ? "mx-3 mt-3 h-14 max-w-3xl rounded-full border border-border-strong bg-surface/90 px-4 shadow-[0_10px_40px_-12px_rgb(0_0_0/0.7)] backdrop-blur-xl supports-[backdrop-filter]:bg-surface/75 sm:mx-auto sm:px-5"
            : "mx-auto mt-0 h-16 max-w-content rounded-none border border-transparent bg-transparent px-4 shadow-none sm:px-6"
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
          <a
            href={signupUrl}
            className={cn(
              buttonVariants({ size: "sm" }),
              "btn-shine glow-brand duration-200 hover:-translate-y-0.5 active:translate-y-0"
            )}
          >
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
          <a
            href={signupUrl}
            onClick={() => setOpen(false)}
            className={cn(
              buttonVariants(),
              "btn-shine glow-brand w-full duration-200 hover:-translate-y-0.5 active:translate-y-0"
            )}
          >
            Start free
          </a>
        </nav>
      </div>
    </header>
  );
}

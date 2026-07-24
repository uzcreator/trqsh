"use client";

import * as React from "react";
import { Menu, Waypoints, X } from "lucide-react";
import { Nav } from "./nav";

// Hamburger + slide-in drawer for the dashboard on phones/tablets. The desktop
// sidebar is hidden below md; this provides the same navigation there.
export function MobileNav() {
  const [open, setOpen] = React.useState(false);

  // Lock body scroll while the drawer is open.
  React.useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  return (
    <div className="md:hidden">
      <button
        type="button"
        aria-label="Open menu"
        aria-expanded={open}
        onClick={() => setOpen(true)}
        className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border-strong bg-surface/60 text-secondary transition-colors hover:bg-accent hover:text-foreground"
      >
        <Menu className="h-4 w-4" />
      </button>

      {open && (
        <div className="fixed inset-0 z-50" role="dialog" aria-modal="true">
          <button
            aria-label="Close menu"
            onClick={() => setOpen(false)}
            className="absolute inset-0 h-full w-full cursor-default animate-fade-in bg-black/60 backdrop-blur-sm"
          />
          <div className="absolute left-0 top-0 flex h-full w-72 max-w-[82%] animate-slide-in-left flex-col border-r border-border bg-surface">
            <div className="flex h-14 items-center justify-between px-5">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                  <Waypoints className="h-4 w-4" />
                </div>
                <span className="font-semibold tracking-tight">trqsh</span>
              </div>
              <button
                type="button"
                aria-label="Close menu"
                onClick={() => setOpen(false)}
                className="inline-flex h-8 w-8 items-center justify-center rounded-md text-secondary transition-colors hover:bg-accent hover:text-foreground"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            {/* A click on any nav link bubbles up here and closes the drawer. */}
            <div className="overflow-y-auto py-2" onClick={() => setOpen(false)}>
              <Nav />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

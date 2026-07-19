"use client";

import * as React from "react";
import type { TocItem } from "@/lib/docs-nav";
import { cn } from "@/lib/utils";

// On-page table of contents with a scroll-spy highlight. Rendered from the TOC
// the server extracted while rendering Markdown, so it needs no DOM parsing.
export function DocsToc({ items }: { items: TocItem[] }) {
  const [activeId, setActiveId] = React.useState<string>("");

  React.useEffect(() => {
    if (items.length === 0) return;
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        }
      },
      { rootMargin: "0px 0px -75% 0px", threshold: 0 }
    );
    for (const item of items) {
      const el = document.getElementById(item.id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [items]);

  if (items.length < 2) return null;

  return (
    <div className="sticky top-24">
      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">On this page</div>
      <ul className="flex flex-col gap-1 border-l border-border">
        {items.map((item) => (
          <li key={item.id} className={item.level === 3 ? "pl-3" : ""}>
            <a
              href={`#${item.id}`}
              className={cn(
                "-ml-px block border-l-2 py-0.5 pl-3 text-sm transition-colors",
                activeId === item.id
                  ? "border-primary font-medium text-primary"
                  : "border-transparent text-secondary hover:text-foreground"
              )}
            >
              {item.text}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}

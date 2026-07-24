"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

const WORD = "trqsh";

// The wordmark *is* the logo — and it types itself out like a command at the
// prompt, then the green terminal caret keeps blinking. It runs once per full
// load (the header lives in the root layout, so navigation doesn't re-trigger it)
// and shows the full word immediately under prefers-reduced-motion.
export function Logo({
  className,
  withCaret = true,
}: {
  className?: string;
  withCaret?: boolean;
}) {
  const [count, setCount] = React.useState(0);

  React.useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setCount(WORD.length);
      return;
    }
    let i = 0;
    const id = window.setInterval(() => {
      i += 1;
      setCount(i);
      if (i >= WORD.length) window.clearInterval(id);
    }, 105);
    return () => window.clearInterval(id);
  }, []);

  return (
    <span className={cn("group inline-flex select-none items-center", className)} aria-label={WORD}>
      <span
        className="text-[1.4rem] font-bold lowercase leading-none tracking-[-0.07em] text-foreground"
        aria-hidden
        suppressHydrationWarning
      >
        {WORD.slice(0, count)}
        {withCaret && (
          <span
            className="wordmark-caret ml-[0.14em] inline-block h-[0.92em] w-[0.14em] translate-y-[0.09em] rounded-[1px] bg-brand align-baseline shadow-[0_0_10px_rgb(var(--brand)/0.7)]"
            aria-hidden
          />
        )}
      </span>
    </span>
  );
}

"use client";

import { cn } from "@/lib/utils";
import { useTypewriterLoop } from "@/lib/use-typewriter-loop";

const WORD = "trqsh";

// The wordmark *is* the logo — and it types itself out like a command at the
// prompt, holds, erases, and retypes on a loop. The green terminal caret
// blinks independently the whole time (.wordmark-caret, always-on infinite
// animation). Shows the full word immediately under prefers-reduced-motion.
export function Logo({
  className,
  withCaret = true,
}: {
  className?: string;
  withCaret?: boolean;
}) {
  const { counts } = useTypewriterLoop([WORD], { typeMs: 105, startDelayMs: 200 });

  return (
    <span className={cn("group inline-flex select-none items-center", className)} aria-label={WORD}>
      <span
        className="text-[1.4rem] font-bold lowercase leading-none tracking-[-0.07em] text-foreground"
        aria-hidden
        suppressHydrationWarning
      >
        {WORD.slice(0, counts[0])}
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

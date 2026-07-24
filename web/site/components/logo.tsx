import { cn } from "@/lib/utils";

// trqsh wordmark — the word *is* the logo. Set tight and heavy in lowercase, with
// a green terminal caret that reads as a command prompt (trqsh is a CLI) and nods
// to the tunnel's "light at the end". The caret blinks only where motion is
// welcome (globals.css disables it under prefers-reduced-motion).
export function Logo({
  className,
  withCaret = true,
}: {
  className?: string;
  withCaret?: boolean;
}) {
  return (
    <span className={cn("group inline-flex select-none items-center", className)}>
      <span className="text-[1.4rem] font-bold lowercase leading-none tracking-[-0.07em] text-foreground">
        trqsh
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

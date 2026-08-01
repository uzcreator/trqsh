"use client";

import { cn } from "@/lib/utils";
import { useTypewriterLoop } from "@/lib/use-typewriter-loop";
import { AuroraTextEffect } from "@/components/lightswind/aurora-text-effect";

const LINE_1 = "Your localhost,";
const LINE_2 = "live on the internet.";

// Types itself out like the wordmark (components/logo.tsx), holds, erases,
// and loops. aria-label carries the real sentence for AX/SEO; the animated
// spans are aria-hidden since they're purely decorative replays of it.
export function HeroHeadline() {
  const { counts, cursorOn } = useTypewriterLoop([LINE_1, LINE_2]);

  return (
    <h1
      className="animate-fade-up text-[2.15rem] font-semibold leading-[1.06] tracking-tight text-foreground sm:text-5xl xl:text-6xl"
      style={{ animationDelay: "90ms" }}
      aria-label={`${LINE_1} ${LINE_2}`}
    >
      <span aria-hidden="true" suppressHydrationWarning>
        <AuroraTextEffect>
          {LINE_1.slice(0, counts[0])}
          <br />
          {LINE_2.slice(0, counts[1])}
        </AuroraTextEffect>
        <span className={cn("type-cursor", cursorOn && "type-cursor-on")} />
      </span>
    </h1>
  );
}

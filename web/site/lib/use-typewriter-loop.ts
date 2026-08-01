"use client";

import * as React from "react";

// Types each line in sequence, holds the fully-typed state, erases back to
// nothing, pauses briefly, then repeats forever. Shared by the wordmark
// (components/logo.tsx) and the hero headline. Falls back to showing the
// full text immediately under prefers-reduced-motion.
export function useTypewriterLoop(
  lines: string[],
  {
    typeMs = 50,
    eraseMs = 25,
    holdMs = 3000,
    gapMs = 350,
    startDelayMs = 300,
  }: {
    typeMs?: number;
    eraseMs?: number;
    holdMs?: number;
    gapMs?: number;
    startDelayMs?: number;
  } = {}
) {
  const key = lines.join("\n");
  const [counts, setCounts] = React.useState<number[]>(() => lines.map(() => 0));
  const [cursorOn, setCursorOn] = React.useState(false);

  React.useEffect(() => {
    const ls = key.split("\n");

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setCounts(ls.map((l) => l.length));
      return;
    }

    let cancelled = false;
    const timeouts: number[] = [];
    const wait = (ms: number) =>
      new Promise<void>((resolve) => {
        timeouts.push(window.setTimeout(resolve, ms));
      });

    (async () => {
      await wait(startDelayMs);
      while (!cancelled) {
        for (let li = 0; li < ls.length; li++) {
          for (let c = 1; c <= ls[li].length; c++) {
            await wait(typeMs);
            if (cancelled) return;
            setCounts((prev) => prev.map((v, i) => (i < li ? ls[i].length : i === li ? c : 0)));
          }
        }
        setCursorOn(true);
        await wait(holdMs);
        if (cancelled) return;
        setCursorOn(false);
        for (let li = ls.length - 1; li >= 0; li--) {
          for (let c = ls[li].length - 1; c >= 0; c--) {
            await wait(eraseMs);
            if (cancelled) return;
            setCounts((prev) => prev.map((v, i) => (i < li ? v : i === li ? c : 0)));
          }
        }
        await wait(gapMs);
      }
    })();

    return () => {
      cancelled = true;
      timeouts.forEach((id) => window.clearTimeout(id));
    };
  }, [key, typeMs, eraseMs, holdMs, gapMs, startDelayMs]);

  return { counts, cursorOn };
}

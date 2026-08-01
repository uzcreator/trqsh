"use client";

import * as React from "react";
import { LineChart, type AnimatedLineProps, type MarkElementProps } from "@mui/x-charts/LineChart";
import { motion } from "motion/react";
import { buttonVariants } from "./ui/button";
import { cn } from "@/lib/utils";

// QUIC-vs-TCP latency visual for the landing page. @mui/x-charts renders the
// axes/grid/geometry (SVG under the hood); custom `line`/`mark` slots swap in
// motion.dev for the draw-in animation. @mui/material is NOT a dependency —
// x-charts v9 doesn't require it, so this stays out of the site's Tailwind-
// only styling approach except for this one chart; the axis/grid colors are
// pinned to the same design tokens the old hand-rolled SVG used, via `sx`.
//
// The numbers are an *illustrative model*, not a measured benchmark — labeled
// as such — showing the real, well-documented effect: TCP suffers head-of-line
// blocking as packet loss rises, while QUIC's independent streams degrade gently.

const LOSS = [0, 1, 2, 3, 4, 5]; // % packet loss
const TCP = [40, 95, 165, 250, 350, 470]; // ms, illustrative
const QUIC = [38, 55, 74, 96, 120, 148]; // ms, illustrative

export function LatencyChart() {
  const [key, replay] = React.useReducer((v: number) => v + 1, 0);

  // x-charts sizes itself via an internal ResizeObserver when no `width` prop
  // is given — inside this grid column (Reveal > grid lg:grid-cols-2), that
  // measured 0 on first read, so the chart's SVG got viewBox="0 0 0 280" and
  // rendered nothing at all (no axes, no ticks, no lines — "the numbers are
  // not viewable" was literally "there is no chart"). Measuring the wrapper
  // ourselves and passing an explicit width sidesteps whatever x-charts'
  // internal observer was getting wrong in this layout.
  const wrapRef = React.useRef<HTMLDivElement>(null);
  const [width, setWidth] = React.useState(0);
  React.useLayoutEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const update = () => setWidth(el.clientWidth);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  return (
    <figure className="rounded-lg border border-border bg-surface p-4 shadow-sm sm:p-5">
      <figcaption className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-semibold text-foreground">
          Median latency as packet loss rises
        </span>
        <span className="flex items-center gap-4 text-xs text-secondary">
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-series-1" /> trqsh (QUIC)
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-series-3" /> TCP tunnel
          </span>
        </span>
      </figcaption>

      <div ref={wrapRef} className="-mx-1">
        {width > 0 && (
        <LineChart
          key={key}
          width={width}
          height={280}
          xAxis={[
            {
              data: LOSS,
              valueFormatter: (v: number) => `${v}%`,
              disableLine: true,
              tickLabelStyle: { fontSize: 11 },
              // Without this, x-charts auto-generates intermediate ticks
              // (0%, 0.5%, 1%, 1.5% ... 5% — 11 labels) that crowd together
              // and become unreadable at this chart's width. Pin ticks to
              // exactly the 6 real data points, matching the original design.
              tickInterval: LOSS,
            },
          ]}
          yAxis={[
            {
              min: 0,
              max: 500,
              disableLine: true,
              tickLabelStyle: { fontSize: 11 },
              tickInterval: [0, 100, 200, 300, 400, 500],
            },
          ]}
          series={[
            {
              data: QUIC,
              curve: "linear",
              color: "rgb(var(--series-1))",
              label: "trqsh (QUIC)",
              showMark: true,
            },
            {
              data: TCP,
              curve: "linear",
              color: "rgb(var(--series-3))",
              label: "TCP tunnel",
              showMark: true,
            },
          ]}
          slots={{ line: AnimatedLine, mark: AnimatedMark }}
          grid={{ horizontal: true }}
          margin={{ left: 36, right: 12, top: 12, bottom: 28 }}
          hideLegend
          sx={{
            ".MuiChartsAxis-tickLabel": { fill: "rgb(var(--muted-ink))" },
            ".MuiChartsAxis-tick": { stroke: "rgb(var(--baseline))" },
            ".MuiChartsGrid-line": { stroke: "rgb(var(--grid))" },
            ".MuiLineElement-root": { strokeWidth: 2.5 },
            "--Charts-axisLabelColor": "rgb(var(--secondary-ink))",
          }}
        />
        )}
      </div>

      <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted">
          Illustrative model of head-of-line blocking, not a measured benchmark. Your mileage varies by
          network and workload.
        </p>
        <button
          type="button"
          onClick={replay}
          className={cn(buttonVariants({ variant: "outline", size: "sm" }), "shrink-0 text-xs")}
        >
          Replay
        </button>
      </div>
    </figure>
  );
}

function AnimatedLine({ d, ownerState, skipAnimation }: AnimatedLineProps) {
  return (
    <motion.path
      d={d}
      fill="transparent"
      stroke={ownerState.color}
      strokeWidth={2.5}
      initial={{ opacity: skipAnimation ? 1 : 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 1.5, ease: "easeInOut" }}
    />
  );
}

function AnimatedMark({ x, y, color, skipAnimation }: MarkElementProps) {
  return (
    <motion.circle
      cx={x}
      cy={y}
      r={3}
      fill={color}
      initial={{ scale: skipAnimation ? 1 : 0, opacity: skipAnimation ? 1 : 0 }}
      animate={{ scale: 1, opacity: 1 }}
      transition={{ duration: 1, delay: 0.5, ease: "backOut" }}
    />
  );
}

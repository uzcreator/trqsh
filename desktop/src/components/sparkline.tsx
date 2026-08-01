import { useMemo } from "react";
import { cn } from "@/lib/utils";

// A dependency-free SVG sparkline for live per-tunnel traffic. Scales to its
// container via preserveAspectRatio="none"; the stroke is kept crisp with
// vector-effect. Renders a soft area fill under the line for depth.

export function Sparkline({
  data,
  className,
  strokeClass = "stroke-wire",
  fillClass = "fill-wire/10",
  height = 28,
}: {
  data: number[];
  className?: string;
  strokeClass?: string;
  fillClass?: string;
  height?: number;
}) {
  const W = 100;
  const H = height;
  const { line, area } = useMemo(() => {
    if (data.length < 2) return { line: "", area: "" };
    const max = Math.max(...data, 1);
    const min = Math.min(...data, 0);
    const span = max - min || 1;
    const step = W / (data.length - 1);
    const pts = data.map((v, i) => {
      const x = i * step;
      const y = H - 2 - ((v - min) / span) * (H - 4);
      return [x, y] as const;
    });
    const line = pts.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
    const area = `${line} L${W},${H} L0,${H} Z`;
    return { line, area };
  }, [data, H]);

  if (!line) {
    return <div className={cn("h-7", className)} style={{ height: H }} aria-hidden />;
  }

  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      preserveAspectRatio="none"
      className={cn("w-full", className)}
      style={{ height: H }}
      aria-hidden
    >
      <path d={area} className={fillClass} stroke="none" />
      <path
        d={line}
        className={strokeClass}
        fill="none"
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}

// QUIC-vs-TCP latency visual for the landing page. Pure SVG (no client JS, no
// image request) so it costs nothing at runtime and scales crisply. It follows
// the dataviz reference palette from the shared tokens: fixed categorical slots
// (series-1 for the highlighted line, series-3 for the comparison), --grid for
// gridlines, --baseline for axes, tabular numerals on ticks.
//
// The numbers are an *illustrative model*, not a measured benchmark — labeled as
// such — showing the real, well-documented effect: TCP suffers head-of-line
// blocking as packet loss rises, while QUIC's independent streams degrade gently.

const LOSS = [0, 1, 2, 3, 4, 5]; // % packet loss
const TCP = [40, 95, 165, 250, 350, 470]; // ms, illustrative
const QUIC = [38, 55, 74, 96, 120, 148]; // ms, illustrative

const W = 640;
const H = 340;
const PAD = { l: 44, r: 16, t: 18, b: 38 };
const PLOT_W = W - PAD.l - PAD.r;
const PLOT_H = H - PAD.t - PAD.b;
const Y_MAX = 500;

const x = (loss: number) => PAD.l + (loss / 5) * PLOT_W;
const y = (ms: number) => PAD.t + (1 - ms / Y_MAX) * PLOT_H;

const path = (data: number[]) =>
  data.map((v, i) => `${i === 0 ? "M" : "L"} ${x(LOSS[i]).toFixed(1)} ${y(v).toFixed(1)}`).join(" ");

export function LatencyChart() {
  const yTicks = [0, 100, 200, 300, 400, 500];

  return (
    <figure className="rounded-lg border border-border bg-surface p-4 shadow-sm sm:p-5">
      <figcaption className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-semibold text-foreground">
          Median latency as packet loss rises
        </span>
        <span className="flex items-center gap-4 text-xs text-secondary">
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-series-1" /> Rift (QUIC)
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-series-3" /> TCP tunnel
          </span>
        </span>
      </figcaption>

      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Latency versus packet loss: QUIC stays low while a TCP tunnel climbs sharply.">
        {/* gridlines + y ticks */}
        {yTicks.map((t) => (
          <g key={t}>
            <line x1={PAD.l} x2={W - PAD.r} y1={y(t)} y2={y(t)} stroke="rgb(var(--grid))" strokeWidth={1} />
            <text x={PAD.l - 8} y={y(t) + 3} textAnchor="end" className="tabular" fill="rgb(var(--muted-ink))" fontSize={11}>
              {t}
            </text>
          </g>
        ))}
        {/* baseline / axes */}
        <line x1={PAD.l} x2={W - PAD.r} y1={y(0)} y2={y(0)} stroke="rgb(var(--baseline))" strokeWidth={1.5} />
        {/* x ticks */}
        {LOSS.map((l) => (
          <text key={l} x={x(l)} y={H - PAD.b + 18} textAnchor="middle" className="tabular" fill="rgb(var(--muted-ink))" fontSize={11}>
            {l}%
          </text>
        ))}

        {/* TCP line (comparison) */}
        <path d={path(TCP)} fill="none" stroke="rgb(var(--series-3))" strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round" />
        {TCP.map((v, i) => (
          <circle key={i} cx={x(LOSS[i])} cy={y(v)} r={3} fill="rgb(var(--series-3))" />
        ))}

        {/* QUIC line (highlight) */}
        <path d={path(QUIC)} fill="none" stroke="rgb(var(--series-1))" strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round" />
        {QUIC.map((v, i) => (
          <circle key={i} cx={x(LOSS[i])} cy={y(v)} r={3} fill="rgb(var(--series-1))" />
        ))}

        {/* axis labels */}
        <text x={PAD.l} y={H - 4} fill="rgb(var(--secondary-ink))" fontSize={11}>
          Packet loss →
        </text>
        <text x={PAD.l - 34} y={PAD.t + 6} fill="rgb(var(--secondary-ink))" fontSize={11}>
          ms
        </text>
      </svg>

      <p className="mt-2 text-xs text-muted">
        Illustrative model of head-of-line blocking, not a measured benchmark. Your mileage varies by
        network and workload.
      </p>
    </figure>
  );
}

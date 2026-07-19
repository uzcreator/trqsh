// A minimal horizontal categorical bar (dataviz): fixed-order series colors,
// 4px rounded data-ends anchored to a recessive track, values direct-labeled so
// identity never rests on color alone. Static by design (a 2–3 bar comparison).

export interface HBarDatum {
  label: string;
  value: number;
  display: string;
  colorVar: string; // e.g. "var(--series-1)"
}

export function HBar({ data }: { data: HBarDatum[] }) {
  const max = Math.max(1, ...data.map((d) => d.value));
  return (
    <div className="flex flex-col gap-3">
      {data.map((d) => (
        <div key={d.label} className="flex items-center gap-3">
          <span className="w-14 shrink-0 text-xs text-secondary">{d.label}</span>
          <div className="relative h-5 flex-1 overflow-hidden rounded-[4px] bg-black/5 dark:bg-white/10">
            <div
              className="h-full rounded-[4px]"
              style={{ width: `${(d.value / max) * 100}%`, backgroundColor: d.colorVar }}
            />
          </div>
          <span className="w-20 shrink-0 text-right text-xs tabular text-secondary">{d.display}</span>
        </div>
      ))}
    </div>
  );
}

export function Legend({ items }: { items: { label: string; colorVar: string }[] }) {
  return (
    <div className="flex flex-wrap items-center gap-4">
      {items.map((it) => (
        <div key={it.label} className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-[3px]" style={{ backgroundColor: it.colorVar }} />
          <span className="text-xs text-secondary">{it.label}</span>
        </div>
      ))}
    </div>
  );
}

"use client";

import * as React from "react";
import dynamic from "next/dynamic";

// The WebGL scene is a separate chunk, loaded only in the browser and only after
// first paint — so three.js never sits in the critical bundle. On reduced-motion
// (and while the chunk loads, and if WebGL is unavailable) a lightweight CSS
// tunnel-glow stands in, keeping the hero instant on slow phones.
const TunnelScene = dynamic(() => import("./tunnel-scene"), {
  ssr: false,
  loading: () => <TunnelGlow />,
});

function TunnelGlow() {
  return <div className="tunnel-fallback absolute inset-0" aria-hidden />;
}

export function Tunnel3D({ className }: { className?: string }) {
  const [enabled, setEnabled] = React.useState(false);

  React.useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    // Phones (< 768px) keep the CSS glow — the three.js chunk isn't even fetched,
    // so first paint stays fast on mobile. Tablets and up get the full scene.
    const bigEnough = window.matchMedia("(min-width: 768px)").matches;
    setEnabled(!reduce && bigEnough);
  }, []);

  return (
    <div className={className}>
      {enabled ? <TunnelScene className="absolute inset-0" /> : <TunnelGlow />}
    </div>
  );
}

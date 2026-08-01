"use client";

import * as React from "react";
import dynamic from "next/dynamic";

// The WebGL scene is a separate chunk, loaded only in the browser and only
// after first paint — same lazy-load approach as components/tunnel-3d.tsx. On
// reduced-motion (and while the chunk loads, and below the lg breakpoint,
// and if WebGL is unavailable) a lightweight CSS glow stands in.
const PlasmaGlobeScene = dynamic(() => import("./plasma-globe-scene"), {
  ssr: false,
  loading: () => <PlasmaGlobeGlow />,
});

function PlasmaGlobeGlow() {
  return <div className="plasma-globe-fallback absolute inset-0" aria-hidden />;
}

export default function PlasmaGlobe({ className }: { className?: string }) {
  const [enabled, setEnabled] = React.useState(false);

  React.useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    // Gated at lg (1024px), a step above Tunnel3D's own md gate — this globe
    // renders *alongside* the hero's Tunnel3D background, so on md/tablet
    // widths running both WebGL scenes at once isn't worth it.
    const bigEnough = window.matchMedia("(min-width: 1024px)").matches;
    setEnabled(!reduce && bigEnough);
  }, []);

  return (
    <div className={className}>
      {enabled ? <PlasmaGlobeScene className="absolute inset-0" /> : <PlasmaGlobeGlow />}
    </div>
  );
}

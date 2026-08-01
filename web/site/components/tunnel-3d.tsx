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
    // The scene itself is already mobile-tuned (fewer rings/segments/packets,
    // capped pixel ratio, no pointer-parallax listener below 768px — see
    // tunnel-scene.tsx's `mobile` checks), so every device gets the real
    // WebGL tunnel. Only reduced-motion (and no-WebGL, caught in
    // tunnel-scene.tsx) fall back to the static CSS glow.
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    setEnabled(!reduce);
  }, []);

  return (
    <div className={className}>
      {enabled ? <TunnelScene className="absolute inset-0" /> : <TunnelGlow />}
    </div>
  );
}

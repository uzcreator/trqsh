"use client";

import * as React from "react";

// The hero's 3D centrepiece: a canvas "tunnel portal". Concentric rings fly
// outward toward the viewer (the feeling of moving through a tunnel) while glowing
// packets stream inward along spokes to a bright core node — localhost being
// carried out to the internet. Green→blue brand palette, DPR-aware, and it parks
// on a single static frame when the visitor prefers reduced motion.

interface Packet {
  angle: number;
  dist: number; // 1 = rim, 0 = core
  speed: number;
  hue: 0 | 1; // 0 green, 1 blue
  size: number;
}

const read = (name: string, fallback: string) => {
  if (typeof window === "undefined") return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
};

export function HeroVisual({ className }: { className?: string }) {
  const canvasRef = React.useRef<HTMLCanvasElement>(null);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const green = `rgb(${read("--brand", "15 172 108")})`;
    const blue = `rgb(${read("--brand-2", "37 78 190")})`;
    const teal = `rgb(${read("--brand-3", "20 184 166")})`;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    let w = 0;
    let h = 0;
    let dpr = 1;
    const parent = canvas.parentElement!;

    const resize = () => {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      w = parent.clientWidth;
      h = parent.clientHeight;
      canvas.width = Math.max(1, Math.floor(w * dpr));
      canvas.height = Math.max(1, Math.floor(h * dpr));
      canvas.style.width = `${w}px`;
      canvas.style.height = `${h}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();

    const RINGS = 7;
    const SPOKES = 32;
    const packets: Packet[] = Array.from({ length: 34 }, () => ({
      angle: Math.random() * Math.PI * 2,
      dist: Math.random(),
      speed: 0.0016 + Math.random() * 0.0034,
      hue: Math.random() > 0.45 ? 0 : 1,
      size: 1 + Math.random() * 2.2,
    }));

    // Pointer parallax — the portal leans a touch toward the cursor.
    let tx = 0;
    let ty = 0;
    let cxOff = 0;
    let cyOff = 0;
    const clamp = (n: number) => Math.max(-1, Math.min(1, n));
    const onPointer = (e: PointerEvent) => {
      const r = parent.getBoundingClientRect();
      tx = clamp(((e.clientX - r.left) / r.width - 0.5) * 2);
      ty = clamp(((e.clientY - r.top) / r.height - 0.5) * 2);
    };
    // Listen on window: the canvas layer is pointer-events-none (behind the card),
    // so parallax should still follow the cursor anywhere over the hero.
    window.addEventListener("pointermove", onPointer);

    let phase = 0;
    let raf = 0;

    const draw = () => {
      const cx = w / 2 + cxOff;
      const cy = h / 2 + cyOff;
      const maxR = Math.min(w, h) * 0.52;

      ctx.clearRect(0, 0, w, h);

      // Core glow.
      const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, maxR * 0.9);
      glow.addColorStop(0, `rgb(${read("--glow", "24 210 140")} / 0.16)`);
      glow.addColorStop(0.5, `rgb(${read("--brand-2", "37 78 190")} / 0.05)`);
      glow.addColorStop(1, "transparent");
      ctx.fillStyle = glow;
      ctx.beginPath();
      ctx.arc(cx, cy, maxR, 0, Math.PI * 2);
      ctx.fill();

      // Concentric rings flying outward (perspective: squashed vertically).
      for (let i = 0; i < RINGS; i++) {
        const t = ((i / RINGS + phase) % 1);
        const r = t * maxR;
        const fade = Math.sin(t * Math.PI); // dim at core and rim
        ctx.save();
        ctx.translate(cx, cy);
        ctx.rotate(phase * 0.6 + i * 0.12);
        ctx.scale(1, 0.62);
        ctx.beginPath();
        // Rounded, superellipse-ish ring echoing the logo's portal tiles.
        ctx.arc(0, 0, r, 0, Math.PI * 2);
        ctx.strokeStyle = green;
        ctx.globalAlpha = 0.14 + fade * 0.4;
        ctx.lineWidth = 1.2 + fade * 1.4;
        ctx.stroke();
        // node dots on the ring
        for (let s = 0; s < SPOKES; s += 4) {
          const a = (s / SPOKES) * Math.PI * 2;
          ctx.beginPath();
          ctx.arc(Math.cos(a) * r, Math.sin(a) * r, 0.8 + fade * 1.1, 0, Math.PI * 2);
          ctx.fillStyle = i % 2 ? teal : green;
          ctx.globalAlpha = fade * 0.5;
          ctx.fill();
        }
        ctx.restore();
      }

      // Packets streaming inward to the core.
      for (const p of packets) {
        p.dist -= p.speed;
        if (p.dist <= 0.02) {
          p.dist = 1;
          p.angle = Math.random() * Math.PI * 2;
          p.hue = Math.random() > 0.45 ? 0 : 1;
        }
        const r = p.dist * maxR;
        const x = cx + Math.cos(p.angle) * r;
        const y = cy + Math.sin(p.angle) * r * 0.62;
        const col = p.hue ? blue : green;
        // trail
        const tr = (p.dist + 0.05) * maxR;
        const x2 = cx + Math.cos(p.angle) * tr;
        const y2 = cy + Math.sin(p.angle) * tr * 0.62;
        ctx.strokeStyle = col;
        ctx.globalAlpha = 0.35 * (1 - p.dist);
        ctx.lineWidth = p.size;
        ctx.beginPath();
        ctx.moveTo(x2, y2);
        ctx.lineTo(x, y);
        ctx.stroke();
        // head
        ctx.globalAlpha = 0.9 * (1 - p.dist * 0.4);
        ctx.fillStyle = col;
        ctx.shadowColor = col;
        ctx.shadowBlur = 10;
        ctx.beginPath();
        ctx.arc(x, y, p.size, 0, Math.PI * 2);
        ctx.fill();
        ctx.shadowBlur = 0;
      }

      // Bright core node.
      ctx.globalAlpha = 1;
      const core = ctx.createRadialGradient(cx, cy, 0, cx, cy, 16);
      core.addColorStop(0, "#ffffff");
      core.addColorStop(0.4, green);
      core.addColorStop(1, "transparent");
      ctx.fillStyle = core;
      ctx.beginPath();
      ctx.arc(cx, cy, 16, 0, Math.PI * 2);
      ctx.fill();

      ctx.globalAlpha = 1;
    };

    const loop = () => {
      phase = (phase + 0.0016) % 1;
      cxOff += (tx * 22 - cxOff) * 0.06;
      cyOff += (ty * 16 - cyOff) * 0.06;
      draw();
      raf = requestAnimationFrame(loop);
    };

    const ro = new ResizeObserver(() => {
      resize();
      if (reduce) draw();
    });
    ro.observe(parent);

    if (reduce) {
      draw();
    } else {
      raf = requestAnimationFrame(loop);
    }

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      window.removeEventListener("pointermove", onPointer);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className={className}
      aria-hidden
      role="presentation"
    />
  );
}

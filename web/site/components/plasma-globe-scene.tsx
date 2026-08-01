"use client";

import * as React from "react";
import * as THREE from "three";

// A glowing wireframe globe beside the hero copy: edge nodes scattered over the
// sphere, arced connections pulsing between them — "your localhost, live on
// the [global] internet" as a picture. Raw three.js (no react-three-fiber) to
// keep the bundle lean, same approach as components/tunnel-scene.tsx; lazy-
// loaded by components/PlasmaGlobe.tsx so it never blocks first paint.

function colorFromVar(name: string, fallback: string): THREE.Color {
  const c = new THREE.Color();
  let val = fallback;
  if (typeof window !== "undefined") {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    if (v) val = v; // "15 172 108"
  }
  const [r, g, b] = val.split(/\s+/).map(Number);
  return c.setRGB(r / 255, g / 255, b / 255, THREE.SRGBColorSpace);
}

function glowTexture(color: THREE.Color): THREE.CanvasTexture {
  const s = 128;
  const cv = document.createElement("canvas");
  cv.width = cv.height = s;
  const ctx = cv.getContext("2d")!;
  const g = ctx.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
  const hex = `#${color.getHexString()}`;
  g.addColorStop(0, "rgba(255,255,255,0.95)");
  g.addColorStop(0.25, hex);
  g.addColorStop(1, "rgba(0,0,0,0)");
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, s, s);
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  return t;
}

// Uniformly-distributed random point on a sphere of radius R.
function spherePoint(R: number): THREE.Vector3 {
  const u = Math.random();
  const v = Math.random();
  const theta = 2 * Math.PI * u;
  const phi = Math.acos(2 * v - 1);
  return new THREE.Vector3(
    R * Math.sin(phi) * Math.cos(theta),
    R * Math.sin(phi) * Math.sin(theta),
    R * Math.cos(phi)
  );
}

export default function PlasmaGlobeScene({ className }: { className?: string }) {
  const wrapRef = React.useRef<HTMLDivElement>(null);
  const canvasRef = React.useRef<HTMLCanvasElement>(null);

  React.useEffect(() => {
    const wrap = wrapRef.current;
    const canvas = canvasRef.current;
    if (!wrap || !canvas) return;

    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true, powerPreference: "high-performance" });
    } catch {
      return; // no WebGL — the CSS fallback in PlasmaGlobe.tsx stays visible
    }

    const mobile = window.matchMedia("(max-width: 768px)").matches;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, mobile ? 1 : 1.6));

    const brand = colorFromVar("--brand", "79 70 229");
    const teal = colorFromVar("--brand-3", "10 147 150");
    const blueC = colorFromVar("--brand-2", "2 62 138");
    const glow = colorFromVar("--glow", "90 70 255");

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(38, 1, 0.1, 50);
    camera.position.set(0, 0, 9);

    const globe = new THREE.Group();
    scene.add(globe);

    const R = 2.6;

    // --- Wireframe sphere: the globe's lat/long grid. ---
    const wireGeo = new THREE.SphereGeometry(R, 28, 18);
    const wireMat = new THREE.MeshBasicMaterial({ color: teal, wireframe: true, transparent: true, opacity: 0.22 });
    globe.add(new THREE.Mesh(wireGeo, wireMat));

    // --- Glowing core, pulsing. ---
    const core = new THREE.Sprite(
      new THREE.SpriteMaterial({ map: glowTexture(glow), blending: THREE.AdditiveBlending, transparent: true, depthWrite: false, opacity: 0.9 })
    );
    core.scale.set(1.6, 1.6, 1);
    globe.add(core);

    // --- Edge nodes scattered on the surface — "points of presence". ---
    const NODES = mobile ? 8 : 14;
    const nodePositions: THREE.Vector3[] = Array.from({ length: NODES }, () => spherePoint(R));
    const nodeGroup = new THREE.Group();
    nodePositions.forEach((p) => {
      const s = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: glowTexture(brand), blending: THREE.AdditiveBlending, transparent: true, depthWrite: false, opacity: 0.95 })
      );
      s.position.copy(p);
      s.scale.set(0.34, 0.34, 1);
      nodeGroup.add(s);
    });
    globe.add(nodeGroup);

    // --- Arcs between random node pairs, bulging outward like flight paths —
    // each carries a traveling pulse sprite to read as data in transit. ---
    interface Arc {
      mat: THREE.LineBasicMaterial;
      pulse: THREE.Sprite;
      curve: THREE.QuadraticBezierCurve3;
      speed: number;
      t: number;
    }
    const arcs: Arc[] = [];
    const ARC_COUNT = mobile ? 6 : 11;
    for (let i = 0; i < ARC_COUNT; i++) {
      const a = nodePositions[Math.floor(Math.random() * nodePositions.length)];
      let b = nodePositions[Math.floor(Math.random() * nodePositions.length)];
      for (let guard = 0; b === a && guard < 5; guard++) {
        b = nodePositions[Math.floor(Math.random() * nodePositions.length)];
      }
      const mid = a.clone().add(b).multiplyScalar(0.5).normalize().multiplyScalar(R * 1.35);
      // QuadraticBezierCurve3, not CatmullRomCurve3: a single arc through one
      // control point is exactly what Bezier curves are for, evaluated by a
      // direct polynomial with no lookback/lookahead. CatmullRomCurve3's
      // default "centripetal" parameterization needs padding points before
      // and after the array to compute segment distances — with only 3 points
      // those padding reads intermittently went out of bounds (undefined),
      // which is what was actually throwing "reading 'x' of undefined".
      const curve = new THREE.QuadraticBezierCurve3(a, mid, b);
      const geo = new THREE.BufferGeometry().setFromPoints(curve.getPoints(40));
      const mat = new THREE.LineBasicMaterial({ color: i % 2 === 0 ? teal : blueC, transparent: true, opacity: 0.32 });
      globe.add(new THREE.Line(geo, mat));

      const pulse = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: glowTexture(glow), blending: THREE.AdditiveBlending, transparent: true, depthWrite: false, opacity: 0.9 })
      );
      pulse.scale.set(0.22, 0.22, 1);
      globe.add(pulse);

      arcs.push({ mat, pulse, curve, speed: 0.15 + Math.random() * 0.18, t: Math.random() });
    }

    // --- Sizing ---
    const resize = () => {
      const w = wrap.clientWidth;
      const h = wrap.clientHeight;
      if (w === 0 || h === 0) return;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    };
    resize();

    // --- Pointer parallax tilt ---
    let tx = 0;
    let ty = 0;
    const onPointer = (e: PointerEvent) => {
      const r = wrap.getBoundingClientRect();
      tx = ((e.clientX - r.left) / r.width - 0.5) * 2;
      ty = ((e.clientY - r.top) / r.height - 0.5) * 2;
    };
    if (!mobile) window.addEventListener("pointermove", onPointer, { passive: true });

    // --- Run only when visible & tab focused ---
    let visible = true;
    const io = new IntersectionObserver(
      (es) => {
        visible = es[0]?.isIntersecting ?? true;
        if (visible && !document.hidden && !reduce && !raf) tick();
      },
      { threshold: 0 }
    );
    io.observe(wrap);
    const onVis = () => {
      if (!document.hidden && visible && !reduce && !raf) tick();
    };
    document.addEventListener("visibilitychange", onVis);
    const ro = new ResizeObserver(resize);
    ro.observe(wrap);

    let raf = 0;
    let t = 0;

    const render = (dt: number) => {
      t += dt;
      const spin = reduce ? 0 : dt * 0.18;
      globe.rotation.y += spin;
      globe.rotation.x += (-ty * 0.3 - globe.rotation.x) * 0.04;
      globe.rotation.z += (tx * 0.12 - globe.rotation.z) * 0.04;

      const coreScale = 1.5 + Math.sin(t * 2.2) * 0.15;
      core.scale.set(coreScale, coreScale, 1);
      core.material.opacity = 0.75 + Math.sin(t * 2.2) * 0.15;

      for (const arc of arcs) {
        arc.t = (arc.t + dt * arc.speed * (reduce ? 0.15 : 1)) % 1;
        // getPoint (simple t-parameterization), not getPointAt (arc-length
        // parameterization): getPointAt builds an internal length lookup table
        // via binary search that, with only 3 control points, intermittently
        // returned an out-of-range result — "reading 'x' of undefined" when
        // .copy() tried to read it. getPoint is a direct evaluation, no table,
        // and a 3-point curve is short enough that the uneven speed doesn't read.
        arc.pulse.position.copy(arc.curve.getPoint(arc.t));
        arc.mat.opacity = 0.26 + Math.sin(t * 3 + arc.t * 6) * 0.08;
      }

      renderer.render(scene, camera);
    };

    let last = performance.now();
    const tick = () => {
      cancelAnimationFrame(raf);
      last = performance.now();
      const loop = (now: number) => {
        const dt = Math.min((now - last) / 1000, 0.05);
        last = now;
        render(dt);
        if (!document.hidden && visible) {
          raf = requestAnimationFrame(loop);
        } else {
          raf = 0; // stopped offscreen / tab hidden; IO + visibilitychange restart it
        }
      };
      raf = requestAnimationFrame(loop);
    };

    if (reduce) {
      render(0); // single static frame
    } else {
      tick();
    }

    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      ro.disconnect();
      document.removeEventListener("visibilitychange", onVis);
      window.removeEventListener("pointermove", onPointer);
      wireGeo.dispose();
      wireMat.dispose();
      core.material.map?.dispose();
      core.material.dispose();
      nodeGroup.children.forEach((n) => {
        const spr = n as THREE.Sprite;
        spr.material.map?.dispose();
        spr.material.dispose();
      });
      globe.children
        .filter((c): c is THREE.Line => c instanceof THREE.Line)
        .forEach((line) => line.geometry.dispose());
      arcs.forEach((a) => {
        a.mat.dispose();
        a.pulse.material.map?.dispose();
        a.pulse.material.dispose();
      });
      renderer.dispose();
    };
  }, []);

  return (
    <div ref={wrapRef} className={className} aria-hidden>
      <canvas ref={canvasRef} className="block h-full w-full" />
    </div>
  );
}

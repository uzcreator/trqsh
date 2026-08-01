"use client";

import * as React from "react";
import * as THREE from "three";

// The trqsh signature: a 3D wormhole you look straight into. Concentric rings
// rush toward you (flying through a tunnel) while brand-coloured packets stream
// down the bore to a glowing core — localhost being carried out to the internet.
// Raw three.js (no react-three-fiber) to keep the bundle lean; lazy-loaded by
// components/tunnel-3d.tsx so it never blocks first paint.

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
  g.addColorStop(0, "rgba(255,255,255,0.9)");
  g.addColorStop(0.2, hex);
  g.addColorStop(1, "rgba(0,0,0,0)");
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, s, s);
  const t = new THREE.CanvasTexture(cv);
  t.colorSpace = THREE.SRGBColorSpace;
  return t;
}

export default function TunnelScene({ className }: { className?: string }) {
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
      return; // no WebGL — the CSS fallback in tunnel-3d.tsx stays visible
    }

    const mobile = window.matchMedia("(max-width: 768px)").matches;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, mobile ? 1 : 1.6));

    const green = colorFromVar("--brand", "79 70 229");
    const teal = colorFromVar("--brand-3", "10 147 150");
    const blue = colorFromVar("--brand-2", "2 62 138");
    const glow = colorFromVar("--glow", "90 70 255");
    const page = colorFromVar("--page", "0 18 25");

    const scene = new THREE.Scene();
    scene.fog = new THREE.FogExp2(page.getHex(), 0.085);

    const camera = new THREE.PerspectiveCamera(72, 1, 0.1, 100);
    camera.position.set(0, 0, 0);

    // --- Rings: one shared circle geometry, N line loops down the bore. ---
    const RINGS = mobile ? 20 : 34;
    const SPACING = 1.7;
    const DEPTH = RINGS * SPACING;
    const RADIUS = 3.4;
    const SEG = mobile ? 48 : 72;
    const circle: THREE.Vector3[] = [];
    for (let i = 0; i < SEG; i++) {
      const a = (i / SEG) * Math.PI * 2;
      circle.push(new THREE.Vector3(Math.cos(a) * RADIUS, Math.sin(a) * RADIUS, 0));
    }
    const ringGeo = new THREE.BufferGeometry().setFromPoints(circle);

    type Ring = { mesh: THREE.LineLoop; mat: THREE.LineBasicMaterial; spin: number };
    const rings: Ring[] = [];
    const ringGroup = new THREE.Group();
    for (let i = 0; i < RINGS; i++) {
      const f = i / RINGS;
      const col = f < 0.5 ? green.clone().lerp(teal, f * 2) : teal.clone().lerp(blue, (f - 0.5) * 2);
      const mat = new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: 0.5 });
      const mesh = new THREE.LineLoop(ringGeo, mat);
      mesh.position.z = -i * SPACING - 2;
      mesh.rotation.z = i * 0.18;
      ringGroup.add(mesh);
      rings.push({ mesh, mat, spin: 0.12 + Math.random() * 0.12 });
    }
    scene.add(ringGroup);

    // --- Packets streaming down the bore. ---
    const COUNT = mobile ? 70 : 150;
    const pos = new Float32Array(COUNT * 3);
    const seed = new Float32Array(COUNT); // radius fraction, for recycle
    for (let i = 0; i < COUNT; i++) {
      const a = Math.random() * Math.PI * 2;
      const rr = (0.15 + Math.random() * 0.85) * RADIUS;
      pos[i * 3] = Math.cos(a) * rr;
      pos[i * 3 + 1] = Math.sin(a) * rr;
      pos[i * 3 + 2] = -Math.random() * DEPTH;
      seed[i] = a;
    }
    const pGeo = new THREE.BufferGeometry();
    pGeo.setAttribute("position", new THREE.BufferAttribute(pos, 3));
    const pMat = new THREE.PointsMaterial({
      color: glow,
      size: mobile ? 0.08 : 0.07,
      sizeAttenuation: true,
      transparent: true,
      opacity: 0.95,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    const packets = new THREE.Points(pGeo, pMat);
    scene.add(packets);

    // --- Light at the end of the tunnel. ---
    const sprite = new THREE.Sprite(
      new THREE.SpriteMaterial({ map: glowTexture(glow), blending: THREE.AdditiveBlending, transparent: true, depthWrite: false, opacity: 0.85 })
    );
    sprite.scale.set(6, 6, 1);
    sprite.position.set(0, 0, -DEPTH * 0.72);
    scene.add(sprite);

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

    // --- Pointer parallax ---
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
    const posAttr = pGeo.getAttribute("position") as THREE.BufferAttribute;

    const render = (dt: number) => {
      t += dt;
      // camera drift + parallax
      camera.position.x += (tx * 0.7 - camera.position.x) * 0.05;
      camera.position.y += (-ty * 0.5 - camera.position.y) * 0.05;
      camera.lookAt(Math.sin(t * 0.1) * 0.4, Math.cos(t * 0.12) * 0.3, -10);

      const speed = dt * (reduce ? 0 : 4.2);
      for (const r of rings) {
        r.mesh.position.z += speed;
        r.mesh.rotation.z += r.spin * dt * 0.4;
        if (r.mesh.position.z > 1.5) r.mesh.position.z -= DEPTH;
        // fade near the camera and into the far fog
        const z = r.mesh.position.z;
        const near = THREE.MathUtils.clamp((1.5 - z) / 3, 0, 1);
        const far = THREE.MathUtils.clamp((z + DEPTH) / 6, 0, 1);
        r.mat.opacity = 0.6 * near * far + 0.08;
      }
      const arr = posAttr.array as Float32Array;
      for (let i = 0; i < COUNT; i++) {
        arr[i * 3 + 2] += speed * 1.15;
        if (arr[i * 3 + 2] > 1.5) {
          arr[i * 3 + 2] -= DEPTH;
          const a = seed[i] + t * 0.2;
          const rr = (0.15 + ((i * 7.3) % 10) / 10 * 0.85) * RADIUS;
          arr[i * 3] = Math.cos(a) * rr;
          arr[i * 3 + 1] = Math.sin(a) * rr;
        }
      }
      posAttr.needsUpdate = true;
      sprite.material.opacity = 0.7 + Math.sin(t * 1.5) * 0.12;

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
      ringGeo.dispose();
      rings.forEach((r) => r.mat.dispose());
      pGeo.dispose();
      pMat.dispose();
      sprite.material.map?.dispose();
      sprite.material.dispose();
      renderer.dispose();
    };
  }, []);

  return (
    <div ref={wrapRef} className={className} aria-hidden>
      <canvas ref={canvasRef} className="block h-full w-full" />
    </div>
  );
}

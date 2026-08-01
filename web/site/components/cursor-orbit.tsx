"use client";

import * as React from "react";

// Replaces the OS cursor with a single canvas shape that morphs between
// three states (idle liquid motion, a text caret, a magnetic snap onto
// hovered elements) — desktop only. `(pointer: fine)` (not a viewport-width
// guess) is what actually means "has a mouse/trackpad, i.e. a cursor to
// replace" — touchscreens report `coarse` and never see this, tablets
// included, regardless of screen size. Toggles .cursor-orbit-active
// (globals.css) so the native cursor only disappears once its replacement is
// actually mounted.
export function CursorOrbit() {
  const [enabled, setEnabled] = React.useState(false);

  React.useEffect(() => {
    const pointerFine = window.matchMedia("(pointer: fine)");
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setEnabled(pointerFine.matches && !reduceMotion.matches);
    update();
    pointerFine.addEventListener("change", update);
    reduceMotion.addEventListener("change", update);
    return () => {
      pointerFine.removeEventListener("change", update);
      reduceMotion.removeEventListener("change", update);
    };
  }, []);

  React.useEffect(() => {
    document.documentElement.classList.toggle("cursor-orbit-active", enabled);
    return () => document.documentElement.classList.remove("cursor-orbit-active");
  }, [enabled]);

  if (!enabled) return null;
  return <CursorShape />;
}

// Any of these (or an explicit opt-in) triggers the magnetic hover state.
// Delegated at the window level so every button/link on every page is
// covered automatically — new components never have to wire this up.
// data-cursor-target is ours; data-magnetic is kept for drop-in
// compatibility with components authored against that convention.
const TARGET_SELECTOR =
  'a, button, [role="button"], input, textarea, select, summary, [data-cursor-target], [data-magnetic]';

// Tags that read as "body text" for the thin text-caret state — mirrors the
// reference brief's own list; anything else styled with `cursor: text`
// (e.g. contenteditable) is caught by the computed-style check below it.
const TEXT_TAGS = new Set(["P", "SPAN", "H1", "H2", "H3", "H4", "H5", "H6"]);

const CURSOR_SIZE = 22; // px, idle/text circle diameter
const HOVER_OUTSET = 8; // px the hover shape extends past the element's box

// POS_EASE is the mouse-follow responsiveness — this is the knob that was
// "particleSpeed: 0.02" on the very first version, where a particle only
// closed 2% of the gap per frame (~2.5s to catch a stationary mouse, which
// read as the cursor being "late"). Kept snappy here for the same reason.
// BOX_EASE/SCALE_EASE are deliberately a bit gentler — the shape morphing
// between idle/text/hover reads as a smooth glide, not a snap.
const POS_EASE = 0.22;
const BOX_EASE = 0.2;
const SCALE_EASE = 0.22;

// Idle squash-and-stretch: how strongly pointer velocity elongates the shape
// along its direction of travel (and thins it perpendicular to that).
const SPEED_SCALE = 0.035;
const MAX_STRETCH = 0.7;
const MAX_SQUASH = 0.4;

/** The shape's box — center + size, so it can morph smoothly between a small circle and an element's bounds. */
interface CursorBox {
  cx: number;
  cy: number;
  w: number;
  h: number;
  radius: number;
}

/** Reads a brand token (space-separated "r g b", see globals.css) for use in canvas fill/stroke strings. */
function brandChannels(name: string, fallback: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
}

function CursorShape() {
  const canvasRef = React.useRef<HTMLCanvasElement>(null);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;

    const brandRgb = brandChannels("--brand", "79 70 229");
    const glowRgb = brandChannels("--glow", "90 70 255");

    const mouse = { x: window.innerWidth / 2, y: window.innerHeight / 2 };
    const pos = { x: mouse.x, y: mouse.y };
    let hoverTarget: HTMLElement | null = null;
    let hoverTargetRadius = 0; // read once per hover-start, not per frame
    let overText = false;
    let lastTextCheckEl: Element | null = null;

    const box: CursorBox = { cx: mouse.x, cy: mouse.y, w: CURSOR_SIZE, h: CURSOR_SIZE, radius: CURSOR_SIZE / 2 };
    let scaleX = 1;
    let scaleY = 1;
    let rotation = 0;

    // --- Sizing: backing-store scaled to devicePixelRatio so the shape stays crisp. ---
    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = window.innerWidth * dpr;
      canvas.height = window.innerHeight * dpr;
      canvas.style.width = `${window.innerWidth}px`;
      canvas.style.height = `${window.innerHeight}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener("resize", resize);

    // --- Pointer tracking + text-caret detection. The text check is skipped
    //     entirely while a magnetic target is active (that always wins) and
    //     only re-evaluated when the hovered element actually changes, not on
    //     every single pointermove — getComputedStyle isn't free. ---
    const onPointerMove = (e: PointerEvent) => {
      mouse.x = e.clientX;
      mouse.y = e.clientY;
      if (hoverTarget) {
        overText = false;
        return;
      }
      const target = e.target as Element | null;
      if (target !== lastTextCheckEl) {
        lastTextCheckEl = target;
        overText = !!target && (TEXT_TAGS.has(target.tagName) || getComputedStyle(target).cursor === "text");
      }
    };
    window.addEventListener("pointermove", onPointerMove, { passive: true });

    // --- Magnetic hover: delegated so any current or future interactive
    //     element on the page is picked up with zero per-component wiring. ---
    const onPointerOver = (e: PointerEvent) => {
      const el = e.target instanceof Element ? e.target.closest<HTMLElement>(TARGET_SELECTOR) : null;
      if (!el || el === hoverTarget) return;
      hoverTarget = el;
      hoverTargetRadius = parseFloat(getComputedStyle(el).borderRadius) || 0;
    };
    const onPointerOut = (e: PointerEvent) => {
      const el = e.target instanceof Element ? e.target.closest<HTMLElement>(TARGET_SELECTOR) : null;
      if (!el || el !== hoverTarget) return;
      const related = e.relatedTarget as Node | null;
      // Moving between two children of the same button (e.g. icon -> label)
      // must not flicker back to idle.
      if (!related || !el.contains(related)) hoverTarget = null;
    };
    window.addEventListener("pointerover", onPointerOver, { passive: true });
    window.addEventListener("pointerout", onPointerOut, { passive: true });

    // --- Pause off-tab: no point animating an invisible cursor. ---
    let visible = !document.hidden;
    const onVisibility = () => {
      visible = !document.hidden;
      if (visible && !raf) raf = requestAnimationFrame(draw);
    };
    document.addEventListener("visibilitychange", onVisibility);

    let raf = 0;
    const draw = () => {
      if (!visible) {
        raf = 0;
        return;
      }
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);

      // Target element must still be on the page — it may have unmounted
      // (route change, modal close, etc.) while the pointer sat over it.
      if (hoverTarget && !hoverTarget.isConnected) hoverTarget = null;
      const rect = hoverTarget?.getBoundingClientRect() ?? null;
      const hovering = !!rect;

      // Position: always eased toward the live mouse — this both places the
      // idle/text shape and, via its own frame-to-frame delta, drives the
      // idle squash-and-stretch below.
      const prevX = pos.x;
      const prevY = pos.y;
      pos.x += (mouse.x - pos.x) * POS_EASE;
      pos.y += (mouse.y - pos.y) * POS_EASE;
      const vx = pos.x - prevX;
      const vy = pos.y - prevY;
      const speed = Math.hypot(vx, vy);

      // Box: a small circle following the pointer, or the hovered element's
      // live bounds (getBoundingClientRect fresh every frame, so scroll/
      // reflow under the cursor is tracked automatically).
      const targetCx = hovering ? rect.left + rect.width / 2 : pos.x;
      const targetCy = hovering ? rect.top + rect.height / 2 : pos.y;
      const targetW = hovering ? rect.width + HOVER_OUTSET * 2 : CURSOR_SIZE;
      const targetH = hovering ? rect.height + HOVER_OUTSET * 2 : CURSOR_SIZE;
      const targetRadius = hovering ? hoverTargetRadius + HOVER_OUTSET : CURSOR_SIZE / 2;
      box.cx += (targetCx - box.cx) * BOX_EASE;
      box.cy += (targetCy - box.cy) * BOX_EASE;
      box.w += (targetW - box.w) * BOX_EASE;
      box.h += (targetH - box.h) * BOX_EASE;
      box.radius += (targetRadius - box.radius) * BOX_EASE;

      // Squash-and-stretch (idle) / thin caret (text) / neutral (hovering —
      // the box morph above does all the work there instead).
      let targetScaleX: number;
      let targetScaleY: number;
      let targetRotation: number;
      if (hovering) {
        targetScaleX = 1;
        targetScaleY = 1;
        // Must ease to 0, not hold — the box above grows to the target's
        // *axis-aligned* width/height, so any leftover rotation from fast
        // approach-velocity makes a wide button render as a tall sideways
        // capsule. The existing eased update already makes this a smooth
        // un-rotate, not a snap — "hold" was solving a problem that easing
        // to 0 already solves, while also never actually recovering.
        targetRotation = 0;
      } else if (overText) {
        targetScaleX = 0.4;
        targetScaleY = 1.8;
        targetRotation = 0;
      } else {
        targetScaleX = 1 + Math.min(speed * SPEED_SCALE, MAX_STRETCH);
        targetScaleY = 1 - Math.min(speed * SPEED_SCALE, MAX_SQUASH);
        // Below ~0.05px/frame the direction is noise (atan2 of a near-zero
        // vector) — hold the last facing instead of jittering.
        targetRotation = speed > 0.05 ? Math.atan2(vy, vx) : rotation;
      }
      scaleX += (targetScaleX - scaleX) * SCALE_EASE;
      scaleY += (targetScaleY - scaleY) * SCALE_EASE;
      rotation += (targetRotation - rotation) * SCALE_EASE;

      const w = Math.max(0, box.w);
      const h = Math.max(0, box.h);
      const r = Math.min(Math.max(0, box.radius), Math.min(w, h) / 2);

      ctx.save();
      ctx.translate(box.cx, box.cy);
      ctx.rotate(rotation);
      ctx.scale(scaleX, scaleY);
      ctx.beginPath();
      ctx.roundRect(-w / 2, -h / 2, w, h, r);
      if (hovering) {
        // Translucent over the button so its label/icon stays legible.
        ctx.fillStyle = `rgb(${brandRgb} / 0.16)`;
        ctx.fill();
        ctx.lineWidth = 1.5 / Math.max(scaleX, 0.01);
        ctx.strokeStyle = `rgb(${glowRgb} / 0.85)`;
        ctx.shadowColor = `rgb(${glowRgb} / 0.6)`;
        ctx.shadowBlur = 16;
        ctx.stroke();
      } else {
        ctx.fillStyle = `rgb(${glowRgb} / 0.92)`;
        ctx.shadowColor = `rgb(${glowRgb} / 0.7)`;
        ctx.shadowBlur = 10;
        ctx.fill();
      }
      ctx.restore();
      ctx.shadowBlur = 0;

      raf = requestAnimationFrame(draw);
    };
    raf = requestAnimationFrame(draw);

    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerover", onPointerOver);
      window.removeEventListener("pointerout", onPointerOut);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="pointer-events-none fixed inset-0 z-50 block h-screen w-screen"
      aria-hidden
    />
  );
}

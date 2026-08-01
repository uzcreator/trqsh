"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

export interface ThreeDCarouselItem {
  // A rendered element, not a component reference — component *references*
  // (e.g. `icon: Download`) aren't serializable across the Server->Client
  // Component boundary when items are built in a server component and this
  // ("use client") carousel just renders them. Render the icon where the
  // items array is built instead: `icon: <Download className="h-5 w-5" />`.
  icon: React.ReactNode;
  title: string;
  body: string;
  code?: string;
  stat?: string;
}

export interface ThreeDCarouselProps {
  items: ThreeDCarouselItem[];
  className?: string;
  /** Auto-rotation speed, in degrees/ms. */
  autoRotateSpeed?: number;
  /** Drag distance -> rotation degrees multiplier. */
  dragSensitivity?: number;
  /** How long after an interaction (drag/focus) before auto-rotate resumes, in ms. */
  resumeDelayMs?: number;
  /** Stage height, in px. Needs headroom for the *tallest* card at its
   *  focused scale (1.14x) — a card with a code block can reach ~280px
   *  unfocused, ~320px focused; the 420px default leaves real margin so nothing
   *  clips against the stage's overflow-hidden edge, top or bottom. */
  height?: number;
}

const DRAG_THRESHOLD = 6; // px of pointer movement before a press counts as a drag
const REST_OPACITY_MIN = 0.8; // sides/back; front is always 1
const FOCUS_SCALE = 1.14;
const FOCUS_DIM_OPACITY = 0.2;

// Each card's angle on the ring, billboarded: rotateY(angle) places it on the
// circle, translateZ(radius) pushes it out to the ring, then rotateY(-angle)
// cancels the first rotation so the card's own face never turns — only its
// position on the circle moves. The carousel rotates; the card doesn't.
// translate(-50%,-50%) must lead the string, not live as a separate Tailwind
// class: an inline style.transform replaces a class-based transform wholesale
// rather than merging with it, so a split "class centers, inline rotates"
// approach silently drops the centering.
function cardTransform(angleDeg: number, radius: number, scale: number): string {
  return `translate(-50%, -50%) rotateY(${angleDeg}deg) translateZ(${radius}px) rotateY(${-angleDeg}deg) scale(${scale})`;
}

// Smooth 0.8 (facing away, at the back) -> 1.0 (facing the viewer, at the
// front) falloff, so the "current" card reads clearly without the rest
// disappearing.
function restOpacity(angleDeg: number): number {
  const rad = (angleDeg * Math.PI) / 180;
  return REST_OPACITY_MIN + (1 - REST_OPACITY_MIN) * ((Math.cos(rad) + 1) / 2);
}

function normalizeAngle(deg: number): number {
  return ((deg % 360) + 360) % 360;
}

/**
 * A ring of real DOM cards arranged with billboarded CSS 3D transforms, not
 * WebGL — text stays crisp, selectable, and natively clickable/keyboard-
 * operable. Auto-rotates via requestAnimationFrame writing straight to each
 * card's DOM node (no per-frame React state, so it doesn't thrash renders);
 * a focus/drag jump gets a short CSS transition layered on top. Pointer
 * Events unify mouse drag and touch swipe under one code path.
 *
 * - Click/tap a card to bring it to front-center and enlarge it.
 * - Drag (mouse or touch) to spin it manually.
 * - Hovering pauses auto-rotate so a click lands on a stable target.
 * - The ring's radius is measured from the stage's real rendered width (via
 *   ResizeObserver), not a guessed breakpoint, so a card at its widest swing
 *   (~90°, pure sideways displacement) always fits inside the stage — nothing
 *   needs to be clipped, at any viewport width.
 */
export default function ThreeDCarousel({
  items,
  className,
  autoRotateSpeed = 0.01,
  dragSensitivity = 0.4,
  resumeDelayMs = 2200,
  height = 420,
}: ThreeDCarouselProps) {
  const n = items.length;
  const angleStep = 360 / n;

  const stageRef = React.useRef<HTMLDivElement>(null);
  const cardRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
  const rotationRef = React.useRef(0);
  const focusedRef = React.useRef<number | null>(null);
  const pausedRef = React.useRef(false);
  const draggingRef = React.useRef(false);
  const dragCommittedRef = React.useRef(false);
  const dragStartX = React.useRef(0);
  const dragStartRotation = React.useRef(0);
  const resumeTimeout = React.useRef<number | undefined>(undefined);

  const [focused, setFocusedState] = React.useState<number | null>(null);
  const [radius, setRadius] = React.useState(280);

  function setFocused(i: number | null) {
    focusedRef.current = i;
    setFocusedState(i);
  }

  const paintCards = React.useCallback(
    (deg: number, animate: boolean) => {
      const focusedIndex = focusedRef.current;
      for (let i = 0; i < n; i++) {
        const el = cardRefs.current[i];
        if (!el) continue;
        const angle = i * angleStep + deg;
        const isFocused = focusedIndex === i;
        const dimmedByFocus = focusedIndex !== null && !isFocused;

        const opacity = isFocused ? 1 : dimmedByFocus ? FOCUS_DIM_OPACITY : restOpacity(angle);
        const scale = isFocused ? FOCUS_SCALE : 1;

        el.style.transition = animate
          ? "transform 0.6s cubic-bezier(0.22,1,0.36,1), opacity 0.4s ease, filter 0.4s ease"
          : "opacity 0.4s ease, filter 0.4s ease";
        el.style.transform = cardTransform(angle, radius, scale);
        el.style.opacity = String(opacity);
        el.style.filter = dimmedByFocus ? "blur(4px)" : "none";
        el.style.zIndex = isFocused ? "10" : "1";
      }
    },
    [n, angleStep, radius]
  );

  const applyRotation = React.useCallback(
    (deg: number, animate: boolean) => {
      rotationRef.current = deg;
      paintCards(deg, animate);
    },
    [paintCards]
  );

  // Radius calibration: measure the stage's real box, not a guessed
  // breakpoint. A card's widest horizontal swing is ~radius (at 90°, pure
  // sideways displacement) plus its own half-width — keeping that under the
  // stage's half-width guarantees no card ever needs clipping.
  React.useLayoutEffect(() => {
    const el = stageRef.current;
    if (!el) return;
    const update = () => {
      const w = el.clientWidth;
      const mobile = window.innerWidth < 640;
      const cardHalf = mobile ? 88 : 120; // half of w-44 / sm:w-60
      // Mobile gets a tighter breathing-room buffer (more of the stage's
      // scarce width goes to ring radius, which is the only lever that turns
      // "cards stacked in a pile" into "cards with a visible gap"); desktop's
      // ceiling is pulled in from 320 so wide screens don't fan out too far.
      const maxRadius = w / 2 - cardHalf - (mobile ? 12 : 20);
      const safeCeiling = Math.min(maxRadius, mobile ? 200 : 270);
      // A floor makes the ring read as intentional rather than collapsed on
      // very narrow screens, but it must never outrank the no-clip ceiling —
      // only apply it when there's room to spare.
      setRadius(safeCeiling > 40 ? Math.max(40, safeCeiling) : Math.max(20, safeCeiling));
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    window.addEventListener("resize", update);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", update);
    };
  }, []);

  // Repaint whenever paintCards itself changes identity (radius/layout changed).
  React.useEffect(() => {
    paintCards(rotationRef.current, false);
  }, [paintCards]);

  React.useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    let raf = 0;
    let last = performance.now();
    const tick = (now: number) => {
      const dt = now - last;
      last = now;
      if (!pausedRef.current && focusedRef.current === null && !draggingRef.current) {
        applyRotation(rotationRef.current + dt * autoRotateSpeed, false);
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [applyRotation, autoRotateSpeed]);

  function resumeSoon() {
    window.clearTimeout(resumeTimeout.current);
    resumeTimeout.current = window.setTimeout(() => {
      pausedRef.current = false;
    }, resumeDelayMs);
  }

  // Pointer handling is deliberately two-stage: a press doesn't commit to
  // being a "drag" (and doesn't call setPointerCapture) until the pointer has
  // moved past DRAG_THRESHOLD. Capturing on every pointerdown — even a plain
  // click — retargets the browser's synthesized click event away from the
  // card button and onto this wrapper, so onClick on the card never fires. A
  // real click (press + release with no meaningful movement) falls through
  // to the button underneath completely untouched.
  function onPointerDown(e: React.PointerEvent) {
    dragStartX.current = e.clientX;
    dragStartRotation.current = rotationRef.current;
    dragCommittedRef.current = false;
  }
  function onPointerMove(e: React.PointerEvent) {
    if (e.buttons === 0 && e.pointerType === "mouse") return;
    const dx = e.clientX - dragStartX.current;
    if (!dragCommittedRef.current) {
      if (Math.abs(dx) < DRAG_THRESHOLD) return;
      dragCommittedRef.current = true;
      draggingRef.current = true;
      pausedRef.current = true;
      setFocused(null);
      (e.currentTarget as Element).setPointerCapture(e.pointerId);
    }
    applyRotation(dragStartRotation.current + dx * dragSensitivity, false);
  }
  function endDrag() {
    if (draggingRef.current) {
      draggingRef.current = false;
      resumeSoon();
    }
    dragCommittedRef.current = false;
  }

  function toggleFocus(i: number) {
    if (focusedRef.current === i) {
      setFocused(null);
      pausedRef.current = false;
      paintCards(rotationRef.current, true);
      return;
    }
    pausedRef.current = true;
    setFocused(i);
    // Card i's current screen angle is (i*angleStep + rotation) mod 360; for
    // that to land on 0 (front-center), the new rotation must be
    // -i*angleStep, not +i*angleStep. The sign was flipped here — it only
    // ever canceled out for i===0 (negating 0 is still 0), which is exactly
    // the card this bug's original test happened to click, so it went
    // unnoticed: every other card focused in place instead of rotating to
    // center.
    const current = normalizeAngle(rotationRef.current);
    const target = normalizeAngle(-i * angleStep);
    let delta = target - current;
    delta = normalizeAngle(delta + 180) - 180;
    applyRotation(rotationRef.current + delta, true);
  }

  return (
    <div
      ref={stageRef}
      className={cn("relative mx-auto max-w-4xl touch-pan-y select-none overflow-hidden", className)}
      style={{ height, perspective: 1400 }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={endDrag}
      onPointerLeave={endDrag}
      onPointerCancel={endDrag}
      onMouseEnter={() => {
        pausedRef.current = true;
      }}
      onMouseLeave={() => {
        if (focusedRef.current === null && !draggingRef.current) pausedRef.current = false;
      }}
    >
      <div className="absolute inset-0" style={{ transformStyle: "preserve-3d" }}>
        {items.map((item, i) => (
          <button
            type="button"
            key={item.title}
            ref={(el) => {
              cardRefs.current[i] = el;
            }}
            aria-label={`${item.title}${focused === i ? " (focused)" : ""}`}
            onClick={() => toggleFocus(i)}
            className={cn(
              "absolute left-1/2 top-1/2 w-44 rounded-xl bg-surface p-4 text-left shadow-[0_20px_50px_-20px_rgb(0_0_0/0.8)] sm:w-60 sm:p-5",
              focused === i && "ring-1 ring-brand/60"
            )}
            style={{
              transform: cardTransform(i * angleStep, radius, 1),
              opacity: restOpacity(i * angleStep),
              backfaceVisibility: "hidden",
            }}
          >
            <span className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-accent text-brand">
              {item.icon}
            </span>
            <h3 className="text-sm font-semibold text-foreground">{item.title}</h3>
            <p className="mt-1.5 text-xs leading-relaxed text-secondary">{item.body}</p>
            {item.code && (
              <code className="mt-3 block overflow-x-auto whitespace-nowrap rounded-md border border-border bg-page px-2.5 py-1.5 font-mono text-[0.68rem] text-brand">
                {item.code}
              </code>
            )}
            {item.stat && (
              <span className="mt-3 inline-block rounded-full bg-accent px-2 py-0.5 font-mono text-[0.65rem] font-medium text-brand">
                {item.stat}
              </span>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

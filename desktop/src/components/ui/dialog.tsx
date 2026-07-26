import { useEffect, useRef } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "./button";

const FOCUSABLE =
  'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

/** Minimal modal — overlay + centered panel, Escape to close, focus trapped
 *  while open and restored to the trigger on close. No portal lib; fixed
 *  positioning is enough for a single-window desktop app. */
export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  className,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: React.ReactNode;
  className?: string;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  // Keep the latest onClose in a ref so the focus-management effect can depend
  // ONLY on `open`. Depending on `onClose` re-ran this whole effect on every
  // parent render (callers pass inline arrows), and the cleanup + re-focus stole
  // focus back to the autoFocus field mid-typing — so characters typed in Name /
  // subdomain / basic-auth landed in the Local address field instead.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    if (!open) return;
    restoreRef.current = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    // Focus the first focusable control (autoFocus inputs handle themselves).
    requestAnimationFrame(() => {
      if (panel && !panel.contains(document.activeElement)) {
        panel.querySelector<HTMLElement>(FOCUSABLE)?.focus();
      }
    });

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onCloseRef.current();
        return;
      }
      if (e.key !== "Tab" || !panel) return;
      const nodes = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
        (n) => n.offsetParent !== null,
      );
      if (nodes.length === 0) return;
      const first = nodes[0];
      const last = nodes[nodes.length - 1];
      const act = document.activeElement as HTMLElement;
      if (e.shiftKey && act === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && act === last) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      restoreRef.current?.focus?.();
    };
  }, [open]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} aria-hidden />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal
        aria-label={title}
        className={cn(
          "dialog-in relative z-10 w-full max-w-md rounded-xl border border-border bg-surface shadow-2xl",
          className,
        )}
      >
        <div className="flex items-start justify-between gap-4 p-4 pb-2">
          <div className="flex flex-col gap-1">
            <h2 className="text-sm font-semibold">{title}</h2>
            {description && <p className="text-xs text-muted">{description}</p>}
          </div>
          <Button variant="ghost" size="icon" className="size-7" onClick={onClose} aria-label="Close">
            <X />
          </Button>
        </div>
        <div className="p-4 pt-2">{children}</div>
      </div>
    </div>
  );
}

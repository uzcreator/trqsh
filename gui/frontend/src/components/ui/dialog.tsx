import { useEffect } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "./button";

/** Minimal modal — overlay + centered panel, Escape to close. No portal lib;
 *  fixed positioning is enough for a single-window desktop app. */
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
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden
      />
      <div
        role="dialog"
        aria-modal
        className={cn(
          "relative z-10 w-full max-w-md rounded-lg border border-border bg-surface shadow-xl",
          className,
        )}
      >
        <div className="flex items-start justify-between gap-4 p-4 pb-2">
          <div className="flex flex-col gap-1">
            <h2 className="text-sm font-semibold">{title}</h2>
            {description && <p className="text-xs text-muted">{description}</p>}
          </div>
          <Button variant="ghost" size="icon" className="size-7" onClick={onClose}>
            <X />
          </Button>
        </div>
        <div className="p-4 pt-2">{children}</div>
      </div>
    </div>
  );
}

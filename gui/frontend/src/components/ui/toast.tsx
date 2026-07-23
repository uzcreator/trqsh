import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Check, Info, TriangleAlert, X } from "lucide-react";
import { cn } from "@/lib/utils";

// A tiny, dependency-free toast system. Transient feedback (copied URL, tunnel
// started, errors, update available) is essential for an app that feels alive;
// the frozen agent core stays untouched — this is pure presentation.

type Tone = "default" | "success" | "error" | "info";

interface ToastItem {
  id: number;
  title: string;
  description?: string;
  tone: Tone;
  duration: number;
}

interface ToastOpts {
  description?: string;
  tone?: Tone;
  /** ms before auto-dismiss; 0 keeps it until dismissed. */
  duration?: number;
}

interface ToastCtx {
  toast: (title: string, opts?: ToastOpts) => number;
  success: (title: string, opts?: ToastOpts) => number;
  error: (title: string, opts?: ToastOpts) => number;
  info: (title: string, opts?: ToastOpts) => number;
  dismiss: (id: number) => void;
}

const Ctx = createContext<ToastCtx | null>(null);

export function useToast(): ToastCtx {
  const c = useContext(Ctx);
  if (!c) throw new Error("useToast must be used within <ToastProvider>");
  return c;
}

const MAX_VISIBLE = 4;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const seq = useRef(0);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  const dismiss = useCallback((id: number) => {
    setItems((x) => x.filter((t) => t.id !== id));
    const t = timers.current.get(id);
    if (t) {
      clearTimeout(t);
      timers.current.delete(id);
    }
  }, []);

  const push = useCallback(
    (title: string, opts?: ToastOpts) => {
      const id = ++seq.current;
      const item: ToastItem = {
        id,
        title,
        description: opts?.description,
        tone: opts?.tone ?? "default",
        duration: opts?.duration ?? 4000,
      };
      setItems((x) => [...x, item].slice(-MAX_VISIBLE));
      if (item.duration > 0) {
        timers.current.set(
          id,
          setTimeout(() => dismiss(id), item.duration),
        );
      }
      return id;
    },
    [dismiss],
  );

  useEffect(() => {
    const t = timers.current;
    return () => t.forEach(clearTimeout);
  }, []);

  const api = useMemo<ToastCtx>(
    () => ({
      toast: push,
      success: (t, o) => push(t, { ...o, tone: "success" }),
      error: (t, o) => push(t, { ...o, tone: "error" }),
      info: (t, o) => push(t, { ...o, tone: "info" }),
      dismiss,
    }),
    [push, dismiss],
  );

  return (
    <Ctx.Provider value={api}>
      {children}
      <div className="pointer-events-none fixed bottom-3 right-3 z-[100] flex w-80 max-w-[calc(100%-1.5rem)] flex-col gap-2">
        {items.map((t) => (
          <ToastCard key={t.id} t={t} onDismiss={() => dismiss(t.id)} />
        ))}
      </div>
    </Ctx.Provider>
  );
}

const toneRing: Record<Tone, string> = {
  default: "text-secondary",
  success: "text-good",
  error: "text-critical",
  info: "text-primary",
};

function ToneIcon({ tone }: { tone: Tone }) {
  const cls = cn("size-4 shrink-0", toneRing[tone]);
  if (tone === "success") return <Check className={cls} />;
  if (tone === "error") return <TriangleAlert className={cls} />;
  return <Info className={cls} />;
}

function ToastCard({ t, onDismiss }: { t: ToastItem; onDismiss: () => void }) {
  return (
    <div
      role="status"
      className="toast-in pointer-events-auto flex items-start gap-2.5 rounded-lg border border-border bg-surface p-3 shadow-lg shadow-black/10"
    >
      <ToneIcon tone={t.tone} />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="text-sm font-medium leading-tight text-foreground">{t.title}</p>
        {t.description && (
          <p className="selectable text-xs leading-snug text-muted">{t.description}</p>
        )}
      </div>
      <button
        onClick={onDismiss}
        className="-m-1 rounded p-1 text-muted transition-colors hover:text-foreground"
        aria-label="Dismiss"
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}

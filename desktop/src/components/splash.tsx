/** Full-window boot / loading screen shown while the Go agent daemon starts and
 *  the session resolves. On a cold start the sidecar takes a moment to bind the
 *  control port, so this replaces the old bare spinner with a branded animation
 *  and keeps the frameless window draggable before the titlebar mounts. */
export function Splash({
  message = "Starting the tunnel engine…",
}: {
  message?: string;
}) {
  return (
    <div
      data-tauri-drag-region
      className="animate-boot flex h-full flex-col items-center justify-center gap-7 bg-page"
    >
      <div className="flex flex-col items-center gap-4">
        <div className="splash-glow flex size-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-[rgb(var(--series-2))]">
          <span className="text-3xl font-bold lowercase text-primary-foreground">t</span>
        </div>
        <div className="flex flex-col items-center gap-1 text-center">
          <span className="text-lg font-semibold tracking-tight">trqsh</span>
          <span className="text-xs text-muted">{message}</span>
        </div>
      </div>
      <div className="h-1 w-40 overflow-hidden rounded-full bg-border">
        <div className="splash-bar h-full w-1/3 rounded-full bg-primary" />
      </div>
    </div>
  );
}

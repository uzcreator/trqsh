import { useEffect, useState } from "react";
import {
  Activity,
  CreditCard,
  Globe,
  KeyRound,
  LogOut,
  Network,
  PanelLeft,
  PanelLeftClose,
  Settings,
  User,
} from "lucide-react";
import { cloud } from "@/lib/agent";
import type { Me } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

export type Screen = "tunnels" | "inspector" | "domains" | "keys" | "billing" | "settings" | "account";

const primaryItems: { id: Screen; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "tunnels", label: "Tunnels", icon: Globe },
  { id: "inspector", label: "Inspector", icon: Activity },
  { id: "domains", label: "Domains", icon: Network },
  { id: "keys", label: "API keys", icon: KeyRound },
  { id: "billing", label: "Billing", icon: CreditCard },
];

const secondaryItems: { id: Screen; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "account", label: "Account", icon: User },
  { id: "settings", label: "Settings", icon: Settings },
];

/** Avatar + "Hello, {name}" header. Fetches its own profile (mirrors how
 *  screens/account.tsx already does its own cloud.me() call) rather than
 *  threading it through App's already-busy Shell state. */
function Profile({ collapsed }: { collapsed: boolean }) {
  const [me, setMe] = useState<Me | null>(null);

  useEffect(() => {
    let alive = true;
    cloud
      .me()
      .then((m) => alive && setMe(m))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  const user = me?.user;
  const name = user?.name || user?.email?.split("@")[0];
  const initial = (user?.name || user?.email || "?").trim().charAt(0).toUpperCase();

  const avatar = user?.avatar_url ? (
    <img
      src={user.avatar_url}
      alt=""
      referrerPolicy="no-referrer"
      className="size-9 shrink-0 rounded-full ring-1 ring-border-strong"
    />
  ) : (
    <div className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/15 text-sm font-semibold text-primary ring-1 ring-primary/25">
      {initial}
    </div>
  );

  if (collapsed) {
    return <div className="flex justify-center px-2 pb-3 pt-1">{avatar}</div>;
  }

  return (
    <div className="flex items-center gap-2.5 px-2.5 pb-4 pt-1">
      {avatar}
      <div className="min-w-0 flex-1">
        <p className="text-[11px] font-medium text-primary">Hello!</p>
        <p className="truncate text-sm font-semibold leading-tight">{name || "Welcome back"}</p>
      </div>
    </div>
  );
}

export function Sidebar({
  active,
  onSelect,
  onDisconnect,
  requestCount,
  collapsed,
  onToggleCollapsed,
}: {
  active: Screen;
  onSelect: (s: Screen) => void;
  onDisconnect: () => void;
  requestCount: number;
  collapsed: boolean;
  onToggleCollapsed: () => void;
}) {
  const [confirmOpen, setConfirmOpen] = useState(false);

  const item = ({ id, label, icon: Icon }: (typeof primaryItems)[number]) => {
    const isActive = active === id;
    const badge = id === "inspector" && requestCount > 0;
    return (
      <button
        key={id}
        onClick={() => onSelect(id)}
        title={collapsed ? label : undefined}
        aria-label={label}
        aria-current={isActive ? "page" : undefined}
        className={cn(
          "relative flex items-center rounded-md text-sm font-medium transition-colors",
          collapsed ? "size-10 justify-center" : "gap-2.5 py-2 pl-3 pr-2.5",
          isActive
            ? "bg-accent text-foreground"
            : "text-secondary hover:bg-accent/60 hover:text-foreground",
        )}
      >
        {isActive && !collapsed && (
          <span className="absolute -left-2 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary" />
        )}
        <Icon className={cn("size-4 shrink-0", isActive && "text-primary")} />
        {!collapsed && <span className="flex-1 text-left">{label}</span>}
        {badge &&
          (collapsed ? (
            <span className="absolute right-1 top-1 size-2 rounded-full bg-primary" />
          ) : (
            <span className="tabular rounded-full bg-primary/15 px-1.5 py-0.5 text-[10px] font-semibold text-primary">
              {requestCount > 999 ? "999+" : requestCount}
            </span>
          ))}
      </button>
    );
  };

  return (
    <nav
      className={cn(
        "flex shrink-0 flex-col border-r border-border bg-surface/60 p-2 transition-[width] duration-200",
        collapsed ? "w-14 items-center" : "w-52",
      )}
    >
      <Profile collapsed={collapsed} />

      <div className="flex flex-col gap-1">{primaryItems.map(item)}</div>

      <div className="my-3 h-px w-full bg-border" />

      <div className="flex flex-col gap-1">{secondaryItems.map(item)}</div>

      <div className="flex-1" />

      <button
        onClick={onToggleCollapsed}
        title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        className={cn(
          "flex items-center rounded-md text-secondary transition-colors hover:bg-accent/60 hover:text-foreground",
          collapsed ? "size-10 justify-center" : "gap-2.5 px-3 py-2",
        )}
      >
        {collapsed ? <PanelLeft className="size-4" /> : <PanelLeftClose className="size-4" />}
        {!collapsed && <span className="text-sm font-medium">Collapse</span>}
      </button>

      <button
        onClick={() => setConfirmOpen(true)}
        title={collapsed ? "Log out" : undefined}
        aria-label="Log out"
        className={cn(
          "mt-1 flex items-center rounded-md text-sm font-semibold text-critical transition-colors hover:bg-critical/10",
          collapsed ? "size-10 justify-center" : "gap-2.5 px-3 py-2",
        )}
      >
        <LogOut className="size-4 shrink-0" />
        {!collapsed && <span>Log out</span>}
      </button>

      <Dialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="Log out?"
        description="Active tunnels will stop and you'll return to sign-in."
      >
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setConfirmOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              setConfirmOpen(false);
              onDisconnect();
            }}
          >
            <LogOut />
            Log out
          </Button>
        </div>
      </Dialog>
    </nav>
  );
}

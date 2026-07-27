import { Activity, Globe, KeyRound, Network, PanelLeft, PanelLeftClose, Settings, User } from "lucide-react";
import { cn } from "@/lib/utils";

export type Screen = "tunnels" | "inspector" | "domains" | "keys" | "settings" | "account";

const items: { id: Screen; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "tunnels", label: "Tunnels", icon: Globe },
  { id: "inspector", label: "Inspector", icon: Activity },
  { id: "domains", label: "Domains", icon: Network },
  { id: "keys", label: "API keys", icon: KeyRound },
  { id: "account", label: "Account", icon: User },
  { id: "settings", label: "Settings", icon: Settings },
];

export function Sidebar({
  active,
  onSelect,
  requestCount,
  collapsed,
  onToggleCollapsed,
}: {
  active: Screen;
  onSelect: (s: Screen) => void;
  requestCount: number;
  collapsed: boolean;
  onToggleCollapsed: () => void;
}) {
  return (
    <nav
      className={cn(
        "flex shrink-0 flex-col gap-1 border-r border-border bg-surface/60 p-2 transition-[width] duration-200",
        collapsed ? "w-14 items-center" : "w-44",
      )}
    >
      {items.map(({ id, label, icon: Icon }) => {
        const isActive = active === id;
        const badge = id === "inspector" && requestCount > 0;
        return (
          <button
            key={id}
            onClick={() => onSelect(id)}
            title={collapsed ? label : undefined}
            aria-label={label}
            className={cn(
              "relative flex items-center rounded-md text-sm font-medium transition-colors",
              collapsed ? "size-10 justify-center" : "gap-2.5 px-3 py-2",
              isActive
                ? "bg-accent text-foreground"
                : "text-secondary hover:bg-accent/60 hover:text-foreground",
            )}
          >
            <Icon className="size-4 shrink-0" />
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
      })}

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
    </nav>
  );
}

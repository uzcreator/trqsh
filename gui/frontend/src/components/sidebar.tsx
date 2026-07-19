import { Activity, Globe, Settings, User } from "lucide-react";
import { cn } from "@/lib/utils";

export type Screen = "tunnels" | "inspector" | "settings" | "account";

const items: { id: Screen; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "tunnels", label: "Tunnels", icon: Globe },
  { id: "inspector", label: "Inspector", icon: Activity },
  { id: "account", label: "Account", icon: User },
  { id: "settings", label: "Settings", icon: Settings },
];

export function Sidebar({
  active,
  onSelect,
  requestCount,
}: {
  active: Screen;
  onSelect: (s: Screen) => void;
  requestCount: number;
}) {
  return (
    <nav className="flex w-44 shrink-0 flex-col gap-1 border-r border-border bg-surface/60 p-2">
      {items.map(({ id, label, icon: Icon }) => {
        const isActive = active === id;
        return (
          <button
            key={id}
            onClick={() => onSelect(id)}
            className={cn(
              "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "bg-accent text-foreground"
                : "text-secondary hover:bg-accent/60 hover:text-foreground",
            )}
          >
            <Icon className="size-4" />
            <span className="flex-1 text-left">{label}</span>
            {id === "inspector" && requestCount > 0 && (
              <span className="tabular rounded-full bg-primary/15 px-1.5 py-0.5 text-[10px] font-semibold text-primary">
                {requestCount > 999 ? "999+" : requestCount}
              </span>
            )}
          </button>
        );
      })}
    </nav>
  );
}

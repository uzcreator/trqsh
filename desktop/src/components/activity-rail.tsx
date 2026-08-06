import { useEffect, useState } from "react";
import { Activity, CreditCard, Globe, KeyRound, Network, Settings } from "lucide-react";
import { cloud } from "@/lib/agent";
import type { Me } from "@/lib/types";
import { cn } from "@/lib/utils";

export type Screen = "tunnels" | "inspector" | "domains" | "keys" | "billing" | "settings" | "account";

const primaryItems: { id: Screen; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "tunnels", label: "Tunnels", icon: Globe },
  { id: "inspector", label: "Inspector", icon: Activity },
  { id: "domains", label: "Domains", icon: Network },
  { id: "keys", label: "API keys", icon: KeyRound },
  { id: "billing", label: "Billing", icon: CreditCard },
];

function RailButton({
  active,
  label,
  onClick,
  children,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      title={label}
      aria-label={label}
      aria-current={active ? "page" : undefined}
      className={cn(
        "relative flex size-10 shrink-0 items-center justify-center rounded-md transition-colors",
        active ? "bg-accent text-foreground" : "text-secondary hover:bg-accent/60 hover:text-foreground",
      )}
    >
      {active && (
        <span className="absolute -left-2 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary" />
      )}
      {children}
    </button>
  );
}

/** Avatar/initials nav button for the Account screen — doubles as the rail's
 *  profile display, so a fixed-width icon rail doesn't also need a separate
 *  "Hello, {name}" block competing for space. Fetches its own profile
 *  (mirrors the pattern screens/account.tsx already uses for cloud.me()). */
function AccountButton({ active, onClick }: { active: boolean; onClick: () => void }) {
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
  const initial = (user?.name || user?.email || "?").trim().charAt(0).toUpperCase();
  const label = user?.name || user?.email || "Account";

  return (
    <RailButton active={active} label={label} onClick={onClick}>
      {user?.avatar_url ? (
        <img
          src={user.avatar_url}
          alt=""
          referrerPolicy="no-referrer"
          className="size-6 rounded-full ring-1 ring-border-strong"
        />
      ) : (
        <div className="flex size-6 items-center justify-center rounded-full bg-primary/15 text-[11px] font-semibold text-primary ring-1 ring-primary/25">
          {initial}
        </div>
      )}
    </RailButton>
  );
}

/** Icon-only primary navigation, VS Code activity-bar style: one fixed width,
 *  no expand/collapse state. The old sidebar's log-out action isn't relocated
 *  here — settings.tsx already has an identical Disconnect control, so this
 *  removal just deletes a duplicate rather than losing functionality. */
export function ActivityRail({
  active,
  onSelect,
  requestCount,
}: {
  active: Screen;
  onSelect: (s: Screen) => void;
  requestCount: number;
}) {
  return (
    <nav className="flex w-14 shrink-0 flex-col items-center gap-1 border-r border-border bg-surface/60 py-3">
      <div className="flex flex-col gap-1">
        {primaryItems.map(({ id, label, icon: Icon }) => {
          const isActive = active === id;
          const badge = id === "inspector" && requestCount > 0;
          return (
            <RailButton key={id} active={isActive} label={label} onClick={() => onSelect(id)}>
              <Icon className={cn("size-[18px]", isActive && "text-primary")} />
              {badge && (
                <span className="absolute right-1.5 top-1.5 size-2 rounded-full bg-primary ring-2 ring-surface" />
              )}
            </RailButton>
          );
        })}
      </div>

      <div className="my-2 h-px w-6 bg-border" />

      <div className="flex flex-col gap-1">
        <AccountButton active={active === "account"} onClick={() => onSelect("account")} />
        <RailButton active={active === "settings"} label="Settings" onClick={() => onSelect("settings")}>
          <Settings className={cn("size-[18px]", active === "settings" && "text-primary")} />
        </RailButton>
      </div>

      <div className="flex-1" />
    </nav>
  );
}

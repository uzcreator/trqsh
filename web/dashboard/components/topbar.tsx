import { LogOut } from "lucide-react";
import { PlanBadge } from "@/components/plan-badge";
import { MobileNav } from "@/components/mobile-nav";
import { logout } from "@/app/actions";
import type { Org, User } from "@/lib/api";

export function Topbar({ org, user }: { org: Org; user: User }) {
  return (
    <header className="sticky top-0 z-10 flex h-14 items-center justify-between gap-2 border-b border-border bg-page/80 px-4 backdrop-blur sm:px-6">
      <div className="flex min-w-0 items-center gap-2">
        <MobileNav />
        <span className="truncate text-sm font-medium">{org.name}</span>
        <PlanBadge plan={org.plan} />
      </div>
      <div className="flex shrink-0 items-center gap-2.5">
        <span className="hidden max-w-[32vw] truncate text-sm text-secondary md:block">{user.email}</span>
        {user.avatar_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={user.avatar_url}
            alt={user.name || user.email}
            referrerPolicy="no-referrer"
            className="h-8 w-8 rounded-full border border-border-strong object-cover"
          />
        ) : (
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-accent text-xs font-semibold text-primary">
            {(user.name || user.email).charAt(0).toUpperCase()}
          </div>
        )}
        <form action={logout}>
          <button
            type="submit"
            title="Log out"
            className="inline-flex h-9 items-center gap-1.5 rounded-md border border-border-strong bg-surface px-3 text-sm text-secondary transition-colors hover:bg-accent hover:text-foreground"
          >
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Log out</span>
          </button>
        </form>
      </div>
    </header>
  );
}

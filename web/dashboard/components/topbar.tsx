import { LogOut } from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { PlanBadge } from "@/components/plan-badge";
import { logout } from "@/app/actions";
import type { Org } from "@/lib/api";

export function Topbar({ org, email }: { org: Org; email: string }) {
  return (
    <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b border-border bg-page/80 px-6 backdrop-blur">
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">{org.name}</span>
        <PlanBadge plan={org.plan} />
      </div>
      <div className="flex items-center gap-2">
        <span className="hidden text-sm text-secondary sm:block">{email}</span>
        <ThemeToggle />
        <form action={logout}>
          <button
            type="submit"
            className="inline-flex h-9 items-center gap-1.5 rounded-md border border-border-strong bg-surface px-3 text-sm text-secondary transition-colors hover:bg-accent"
          >
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Log out</span>
          </button>
        </form>
      </div>
    </header>
  );
}

import { redirect } from "next/navigation";
import { Waypoints } from "lucide-react";
import { api, type Account } from "@/lib/api";
import { Nav } from "@/components/nav";
import { Topbar } from "@/components/topbar";
import { PageTransition } from "@/components/page-transition";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  let account: Account;
  try {
    account = await api.account();
  } catch {
    redirect("/login");
  }

  const org =
    account.orgs.find((o) => o.id === account.active_org) ??
    account.orgs[0] ?? { id: "", name: "Your org", plan: "free", created_at: "" };

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-60 shrink-0 flex-col border-r border-border bg-surface md:flex">
        <div className="flex h-14 items-center gap-2 px-5">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Waypoints className="h-4 w-4" />
          </div>
          <span className="font-semibold tracking-tight">Rift</span>
        </div>
        <div className="py-2">
          <Nav />
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar org={org} email={account.user.email} />
        <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
          <PageTransition>{children}</PageTransition>
        </main>
      </div>
    </div>
  );
}

import { api, safe } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PlanBadge } from "@/components/plan-badge";
import { logout } from "@/app/actions";
import { formatDate } from "@/lib/format";

export default async function SettingsPage() {
  const account = await safe(api.account());
  const user = account?.user;
  const org = account?.orgs.find((o) => o.id === account.active_org) ?? account?.orgs[0];

  return (
    <div className="max-w-2xl">
      <PageHeader title="Settings" description="Your account and organization details." />

      <div className="flex flex-col gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Account</CardTitle>
            <CardDescription>Signed in as {user?.email}.</CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-y-4 text-sm">
              <Field label="Name" value={user?.name || "—"} />
              <Field label="Email" value={user?.email || "—"} />
              <Field label="Sign-in method" value={user?.oauth_provider || "email"} />
              <Field label="Member since" value={formatDate(user?.created_at)} />
            </dl>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Organization</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-y-4 text-sm">
              <Field label="Name" value={org?.name || "—"} />
              <Field label="Plan" value={<PlanBadge plan={org?.plan ?? "free"} />} />
              <Field label="Organization ID" value={<span className="font-mono text-xs">{org?.id}</span>} />
              <Field label="Created" value={formatDate(org?.created_at)} />
            </dl>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Session</CardTitle>
            <CardDescription>Sign out of the dashboard on this device.</CardDescription>
          </CardHeader>
          <CardContent>
            <form action={logout}>
              <Button variant="outline" type="submit">
                Log out
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="mt-0.5 font-medium">{value}</dd>
    </div>
  );
}

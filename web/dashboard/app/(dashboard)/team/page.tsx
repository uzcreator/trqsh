import Link from "next/link";
import { Crown, UserPlus } from "lucide-react";
import { api, safe } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export default async function TeamPage() {
  const [account, subs] = await Promise.all([safe(api.account()), safe(api.subscription())]);
  const plan = subs?.plan ?? "free";
  const isTeam = plan === "team";
  const user = account?.user;
  const org = account?.orgs.find((o) => o.id === account.active_org) ?? account?.orgs[0];

  return (
    <div>
      <PageHeader
        title="Team"
        description="Members and roles for your organization."
        action={
          isTeam ? (
            <Button disabled>
              <UserPlus className="h-4 w-4" /> Invite
            </Button>
          ) : undefined
        }
      />

      {!isTeam && (
        <Card className="mb-6">
          <CardContent className="flex flex-wrap items-center justify-between gap-4 p-5">
            <div>
              <p className="font-medium">Collaborate with your team</p>
              <p className="mt-1 text-sm text-secondary">
                Invites, roles, and SSO/SAML are part of the Team plan.
              </p>
            </div>
            <Link href="/billing">
              <Button>Upgrade to Team</Button>
            </Link>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>{org?.name ?? "Your organization"}</CardTitle>
          <CardDescription>People with access to this organization.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="divide-y divide-border">
            <div className="flex items-center justify-between py-3">
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-accent text-sm font-semibold text-primary">
                  {(user?.name || user?.email || "?").slice(0, 1).toUpperCase()}
                </div>
                <div>
                  <p className="text-sm font-medium">{user?.name || "You"}</p>
                  <p className="text-xs text-secondary">{user?.email}</p>
                </div>
              </div>
              <Badge variant="default">
                <Crown className="h-3 w-3" /> Owner
              </Badge>
            </div>
          </div>
        </CardContent>
      </Card>

      <p className="mt-4 text-xs text-muted">
        Member management (invites, roles, SSO) is exposed once the control API adds team endpoints; the
        data model already supports org members and roles.
      </p>
    </div>
  );
}

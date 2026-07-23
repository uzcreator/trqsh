import { api, safe } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { DomainsManager } from "./domains-manager";

const BASE_DOMAIN = process.env.NEXT_PUBLIC_TRQSH_BASE_DOMAIN || "trqsh.uz";

export default async function DomainsPage() {
  const [subdomains, domains] = await Promise.all([safe(api.subdomains()), safe(api.domains())]);
  return (
    <div>
      <PageHeader title="Domains" description="Reserved subdomains and custom domains for your tunnels." />
      <DomainsManager subdomains={subdomains ?? []} domains={domains ?? []} baseDomain={BASE_DOMAIN} />
    </div>
  );
}

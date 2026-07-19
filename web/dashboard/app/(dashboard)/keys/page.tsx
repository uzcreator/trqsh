import { api, safe } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { KeysManager } from "./keys-manager";

export default async function KeysPage() {
  const keys = (await safe(api.apiKeys())) ?? [];
  // Newest first.
  keys.sort((a, b) => (a.created_at < b.created_at ? 1 : -1));
  return (
    <div>
      <PageHeader title="API Keys" description="Credentials the agent and GUI use to authenticate." />
      <KeysManager keys={keys} />
    </div>
  );
}

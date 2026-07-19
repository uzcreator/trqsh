import { Badge } from "@/components/ui/badge";

const LABELS: Record<string, string> = {
  free: "Free",
  pro: "Pro",
  team: "Team",
  payg: "Pay-as-you-go",
};

export function PlanBadge({ plan }: { plan: string }) {
  const label = LABELS[plan] ?? plan;
  return <Badge variant={plan === "free" ? "muted" : "default"}>{label}</Badge>;
}

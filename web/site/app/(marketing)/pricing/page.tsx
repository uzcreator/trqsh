import type { Metadata } from "next";
import { PricingTable } from "@/components/pricing-table";

export const metadata: Metadata = {
  title: "Pricing",
  description:
    "Simple, predictable pricing for Rift — a generous free tier, Pro, and Team plans. The prices shown are the exact limits our edge enforces.",
};

const FAQ = [
  {
    q: "Is the free tier really free?",
    a: "Yes — free forever, no credit card. It's deliberately more generous than ngrok's 2026 tier, and the limits you see are the ones the edge actually enforces.",
  },
  {
    q: "What counts as bandwidth?",
    a: "Total bytes proxied through your tunnels in a calendar month, in and out. Usage resets on the first of each month; you can watch it live in the dashboard.",
  },
  {
    q: "Can I change plans anytime?",
    a: "Yes. Upgrades take effect immediately via Stripe Checkout; downgrades and cancellations are handled in the Stripe Customer Portal and apply at the end of the period.",
  },
  {
    q: "What happens when I hit a limit?",
    a: "New binds that would exceed a quota are declined with a clear error (and the plan that lifts it), so nothing breaks silently. Existing tunnels keep serving.",
  },
  {
    q: "Do you offer Pay-as-you-go?",
    a: "Yes — a metered plan bills only for what you use, ideal for spiky or automated workloads. Reach out and we'll set you up.",
  },
];

export default function PricingPage() {
  return (
    <div className="mx-auto max-w-content px-4 py-16 sm:px-6">
      <header className="mx-auto mb-12 max-w-2xl text-center">
        <h1 className="text-4xl font-semibold tracking-tight text-foreground">
          Pricing that scales with you
        </h1>
        <p className="mt-4 text-lg text-secondary">
          Start free, upgrade when you outgrow it. Every number below is pulled straight from our
          billing catalog — the same limits the edge enforces, so nothing here can drift.
        </p>
      </header>

      <PricingTable />

      <section className="mx-auto mt-20 max-w-3xl">
        <h2 className="mb-6 text-center text-2xl font-semibold tracking-tight text-foreground">
          Frequently asked questions
        </h2>
        <div className="flex flex-col gap-3">
          {FAQ.map((item) => (
            <details
              key={item.q}
              className="group rounded-lg border border-border bg-surface p-5 shadow-sm"
            >
              <summary className="cursor-pointer list-none font-medium text-foreground marker:content-none">
                <span className="flex items-center justify-between gap-4">
                  {item.q}
                  <span className="text-muted transition-transform group-open:rotate-45">+</span>
                </span>
              </summary>
              <p className="mt-3 text-sm text-secondary">{item.a}</p>
            </details>
          ))}
        </div>
      </section>
    </div>
  );
}

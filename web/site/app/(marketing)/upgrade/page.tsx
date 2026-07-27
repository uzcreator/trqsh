import type { Metadata } from "next";
import Link from "next/link";
import { Send, Check, ArrowLeft } from "lucide-react";
import { site } from "@/lib/site";
import { planByCode } from "@/lib/plans";
import { formatPrice } from "@/lib/format";

export const metadata: Metadata = {
  title: "Upgrade",
  description: "Upgrade your trqsh plan — message us on Telegram and we'll activate it.",
};

export default async function UpgradePage({
  searchParams,
}: {
  searchParams: Promise<{ plan?: string; cadence?: string }>;
}) {
  const sp = await searchParams;
  const code = sp.plan === "team" ? "team" : "pro";
  const cadence = sp.cadence === "annual" ? "annual" : "monthly";
  const plan = planByCode(code);
  const name = plan?.name ?? "Pro";

  const monthlyCents =
    cadence === "annual"
      ? Math.round((plan?.price_annual_cents ?? 0) / 12)
      : (plan?.price_monthly_cents ?? 0);

  const message = `Hi! I'd like the ${name} plan (${cadence === "annual" ? "yearly" : "monthly"}) for trqsh. My account email is: `;
  const tgWithText = `${site.telegramUrl}?text=${encodeURIComponent(message)}`;

  const steps = [
    <>
      Message us on Telegram at{" "}
      <a href={site.telegramUrl} className="font-medium text-primary hover:underline">
        @{site.telegram}
      </a>
      .
    </>,
    <>Tell us your trqsh account email and the plan you want ({name}, {cadence === "annual" ? "yearly" : "monthly"}).</>,
    <>We activate your subscription — usually within a few hours — and it shows up in the app.</>,
  ];

  return (
    <div className="mx-auto max-w-content px-4 py-16 sm:px-6">
      <div className="mx-auto max-w-xl">
        <Link
          href="/pricing"
          className="mb-8 inline-flex items-center gap-1.5 text-sm text-secondary transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to pricing
        </Link>

        <header className="mb-8">
          <h1 className="text-3xl font-semibold tracking-tight text-foreground">Upgrade to {name}</h1>
          <p className="mt-3 text-secondary">
            We don&apos;t take card payments yet — upgrades are handled personally over Telegram. It only
            takes a minute.
          </p>
        </header>

        <div className="mb-6 flex items-center justify-between rounded-lg border border-primary/30 bg-primary/5 p-5">
          <div>
            <p className="text-sm text-secondary">{name} plan</p>
            <p className="mt-0.5 text-2xl font-semibold tracking-tight text-foreground tabular">
              {formatPrice(monthlyCents)}
              <span className="text-sm font-normal text-muted">/mo</span>
            </p>
            {cadence === "annual" && plan && (
              <p className="text-xs text-muted tabular">billed {formatPrice(plan.price_annual_cents)}/yr</p>
            )}
          </div>
          <span className="rounded-full bg-primary/15 px-2.5 py-1 text-xs font-semibold text-primary">
            {cadence === "annual" ? "Yearly" : "Monthly"}
          </span>
        </div>

        <a
          href={tgWithText}
          target="_blank"
          rel="noopener noreferrer"
          className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-5 py-3 font-medium text-primary-foreground transition-opacity hover:opacity-90"
        >
          <Send className="h-4 w-4" />
          Message @{site.telegram} on Telegram
        </a>

        <ol className="mt-8 flex flex-col gap-4">
          {steps.map((s, i) => (
            <li key={i} className="flex items-start gap-3">
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-semibold text-primary">
                {i + 1}
              </span>
              <span className="text-sm text-secondary">{s}</span>
            </li>
          ))}
        </ol>

        <div className="mt-8 flex items-start gap-2 rounded-lg border border-border bg-surface p-4 text-sm text-secondary">
          <Check className="mt-0.5 h-4 w-4 shrink-0 text-good" />
          <span>
            Include the <span className="font-medium text-foreground">email you signed in with</span> so we
            can find your account. That&apos;s the only detail we need.
          </span>
        </div>
      </div>
    </div>
  );
}

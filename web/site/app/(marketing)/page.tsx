import Link from "next/link";
import {
  ArrowRight,
  Gauge,
  Gift,
  GitBranch,
  Globe,
  Lock,
  MonitorSmartphone,
  Radio,
  Repeat,
  Bookmark,
  Signal,
  Webhook,
  Terminal as TerminalIcon,
} from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Terminal } from "@/components/terminal";
import { Reveal } from "@/components/reveal";
import { InstallTabs } from "@/components/install-tabs";
import { LatencyChart } from "@/components/latency-chart";
import { ComparisonTable } from "@/components/comparison-table";
import { site, signupUrl } from "@/lib/site";
import { planByCode } from "@/lib/plans";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

const free = planByCode("free");

const DIFFERENTIATORS = [
  {
    icon: Gauge,
    title: "QUIC-first, so it's fast",
    body: "Tunnels ride QUIC/HTTP-3 by default — lower latency on lossy and mobile links, and connection migration when you hop Wi-Fi ↔ 5G. Blocked UDP? It falls back to TCP automatically.",
  },
  {
    icon: Gift,
    title: "A free tier that isn't a trap",
    body: free
      ? `${formatBytes(free.max_bandwidth_bytes_mo)} of transfer, ${free.max_concurrent_tunnels} concurrent tunnels, and a reserved subdomain — free forever. Deliberately more generous than ngrok's 2026 tier.`
      : "Generous limits, free forever — deliberately more generous than ngrok's 2026 tier.",
  },
  {
    icon: MonitorSmartphone,
    title: "A real desktop app",
    body: "Not just a CLI. A polished native app for macOS, Windows, and Linux with one-click tunnels, a live request inspector, replay, and a system tray.",
  },
  {
    icon: Radio,
    title: "Every protocol, including UDP",
    body: "HTTP, HTTPS, TLS, raw TCP — and UDP, which ngrok simply doesn't do. Tunnel game servers, DNS, WebRTC, and QUIC services, not just web apps.",
  },
  {
    icon: Globe,
    title: "Custom domains & teams",
    body: "Instant reserved subdomains, bring-your-own custom domains with guided DNS, and shared team domains with SSO/SAML on the Team plan.",
  },
  {
    icon: GitBranch,
    title: "Open-source agent",
    body: "The agent you run is open source — audit it, script it, self-host it. No black box on your machine. Trust through transparency.",
  },
];

const SECONDARY = [
  { icon: Bookmark, title: "Reserved subdomains", body: "Keep the same URL across restarts." },
  { icon: Repeat, title: "Request replay", body: "Re-fire any captured request while you debug." },
  { icon: Lock, title: "Basic auth & schemes", body: "Protect a tunnel with a username/password in one flag." },
  { icon: Webhook, title: "Webhooks & CI", body: "Receive provider webhooks and share preview builds." },
  { icon: Signal, title: "Connection migration", body: "Network changes don't drop your tunnel." },
  { icon: TerminalIcon, title: "Scriptable CLI", body: "`rift http 3000`, config files, and exit codes for automation." },
];

export default function LandingPage() {
  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden border-b border-border">
        <div className="hero-grid pointer-events-none absolute inset-0" aria-hidden />
        <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden>
          <div className="animate-blob absolute -left-24 -top-10 h-72 w-72 rounded-full bg-series-1/20 blur-3xl" />
          <div
            className="animate-blob absolute right-0 top-6 h-80 w-80 rounded-full bg-series-5/20 blur-3xl"
            style={{ animationDelay: "-6s" }}
          />
        </div>
        <div className="relative mx-auto max-w-content px-4 pb-16 pt-16 sm:px-6 sm:pt-24">
          <div className="grid items-center gap-12 lg:grid-cols-2">
            <div className="animate-fade-up">
              <Badge variant="default" className="mb-5">
                <Signal className="h-3.5 w-3.5" /> Now running on QUIC / HTTP-3
              </Badge>
              <h1 className="text-4xl font-semibold tracking-tight text-foreground sm:text-5xl">
                Your localhost, <span className="gradient-text">live on the internet.</span>
              </h1>
              <p className="mt-5 max-w-xl text-lg text-secondary">
                Rift exposes a local port to a public HTTPS URL in seconds — over QUIC for lower
                latency, with UDP support, a desktop app, and a free tier that stays out of your way.
              </p>
              <div className="mt-7 flex flex-wrap items-center gap-3">
                <a href={signupUrl} className={cn(buttonVariants({ size: "xl" }), "btn-shine")}>
                  Start free <ArrowRight className="h-4 w-4" />
                </a>
                <Link href="/download" className={cn(buttonVariants({ variant: "outline", size: "xl" }))}>
                  Download the app
                </Link>
              </div>
              <div className="mt-8 max-w-md">
                <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted">
                  Or install the CLI
                </p>
                <InstallTabs />
              </div>
            </div>

            <div className="animate-fade-up lg:pl-4">
              <Terminal />
              <p className="mt-3 text-center text-sm text-muted">
                One command. A live URL, TLS, and an inspector — no signup required to try.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Trust strip */}
      <section className="border-b border-border bg-surface">
        <div className="mx-auto flex max-w-content flex-wrap items-center justify-center gap-x-8 gap-y-2 px-4 py-5 text-sm text-muted sm:px-6">
          <span>Open-source agent</span>
          <span className="text-border-strong">•</span>
          <span>QUIC / HTTP-3 transport</span>
          <span className="text-border-strong">•</span>
          <span>HTTP · TCP · UDP</span>
          <span className="text-border-strong">•</span>
          <span>macOS · Windows · Linux</span>
          <span className="text-border-strong">•</span>
          <span>Custom domains &amp; teams</span>
        </div>
      </section>

      {/* Differentiators */}
      <Section
        eyebrow="Why Rift"
        title="Built to be the fastest, friendliest way to share localhost"
        subtitle="Every feature protects one of five promises: speed, a generous free tier, a great GUI, every protocol, and an open agent."
      >
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {DIFFERENTIATORS.map((f, i) => (
            <Reveal key={f.title} delay={(i % 3) * 90} className="h-full">
              <FeatureCard icon={f.icon} title={f.title} body={f.body} />
            </Reveal>
          ))}
        </div>
      </Section>

      {/* Speed / QUIC */}
      <section className="border-y border-border bg-surface">
        <div className="mx-auto max-w-content px-4 py-20 sm:px-6">
          <div className="grid items-center gap-12 lg:grid-cols-2">
            <div>
              <Badge variant="default" className="mb-4">
                <Gauge className="h-3.5 w-3.5" /> Speed
              </Badge>
              <h2 className="text-3xl font-semibold tracking-tight text-foreground">
                QUIC keeps you fast where TCP falls apart
              </h2>
              <p className="mt-4 text-secondary">
                A dropped packet on a TCP tunnel stalls <em>everything</em> behind it — head-of-line
                blocking. QUIC carries each stream independently, so one lost packet doesn&apos;t
                freeze the rest. On a flaky café Wi-Fi or a train, that&apos;s the difference between
                snappy and unusable.
              </p>
              <ul className="mt-5 flex flex-col gap-2 text-sm text-secondary">
                <li className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 rounded-full bg-series-1" /> Independent streams — no
                  head-of-line blocking
                </li>
                <li className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 rounded-full bg-series-1" /> Connection migration across
                  network changes
                </li>
                <li className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 rounded-full bg-series-1" /> Automatic TCP + yamux
                  fallback where UDP is blocked
                </li>
              </ul>
            </div>
            <LatencyChart />
          </div>
        </div>
      </section>

      {/* Secondary features */}
      <Section
        eyebrow="And the essentials"
        title="Everything you'd expect, done well"
        subtitle="The small things that make a tunnel pleasant to live in."
      >
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {SECONDARY.map((f, i) => (
            <Reveal key={f.title} delay={(i % 3) * 80} className="h-full">
              <div className="card-hover flex h-full gap-3 rounded-lg border border-border bg-surface p-5">
                <f.icon className="h-5 w-5 shrink-0 text-primary" />
                <div>
                  <h3 className="text-sm font-semibold text-foreground">{f.title}</h3>
                  <p className="mt-1 text-sm text-secondary">{f.body}</p>
                </div>
              </div>
            </Reveal>
          ))}
        </div>
      </Section>

      {/* Comparison */}
      <section className="border-y border-border bg-surface">
        <div className="mx-auto max-w-content px-4 py-20 sm:px-6">
          <div className="mx-auto mb-10 max-w-2xl text-center">
            <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-primary">Honest comparison</p>
            <h2 className="text-3xl font-semibold tracking-tight text-foreground">
              How Rift stacks up
            </h2>
            <p className="mt-3 text-secondary">
              We&apos;ll give credit where it&apos;s due — and show you exactly where Rift pulls ahead.
            </p>
          </div>
          <ComparisonTable />
        </div>
      </section>

      {/* Pricing teaser */}
      <Section
        eyebrow="Pricing"
        title="Start free. Upgrade when you outgrow it."
        subtitle="Simple, predictable plans — the same numbers our edge actually enforces."
      >
        <div className="mx-auto flex max-w-3xl flex-col items-center gap-6 rounded-xl border border-border bg-surface p-8 text-center shadow-sm">
          {free && (
            <div className="grid w-full grid-cols-2 gap-4 sm:grid-cols-4">
              <Stat value={formatBytes(free.max_bandwidth_bytes_mo)} label="Free bandwidth / mo" />
              <Stat value={String(free.max_concurrent_tunnels)} label="Concurrent tunnels" />
              <Stat value={String(free.max_reserved_subdomains)} label="Reserved subdomain" />
              <Stat value="$0" label="Forever" />
            </div>
          )}
          <div className="flex flex-wrap justify-center gap-3">
            <a href={signupUrl} className={cn(buttonVariants({ size: "lg" }))}>
              Start free
            </a>
            <Link href="/pricing" className={cn(buttonVariants({ variant: "outline", size: "lg" }))}>
              See all plans <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </Section>

      {/* Final CTA */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-content px-4 py-20 text-center sm:px-6">
          <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            Ship your localhost in 60 seconds
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-secondary">
            Install the CLI or grab the desktop app, run one command, and share a live URL.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <a href={signupUrl} className={cn(buttonVariants({ size: "xl" }))}>
              Start free <ArrowRight className="h-4 w-4" />
            </a>
            <Link href="/docs/quickstart" className={cn(buttonVariants({ variant: "outline", size: "xl" }))}>
              Read the quickstart
            </Link>
          </div>
        </div>
      </section>
    </>
  );
}

function Section({
  eyebrow,
  title,
  subtitle,
  children,
}: {
  eyebrow: string;
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="mx-auto max-w-content px-4 py-20 sm:px-6">
      <Reveal className="mx-auto mb-10 max-w-2xl text-center">
        <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-primary">{eyebrow}</p>
        <h2 className="text-3xl font-semibold tracking-tight text-foreground">{title}</h2>
        {subtitle && <p className="mt-3 text-secondary">{subtitle}</p>}
      </Reveal>
      {children}
    </section>
  );
}

function FeatureCard({
  icon: Icon,
  title,
  body,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  body: string;
}) {
  return (
    <div className="card-hover group h-full rounded-lg border border-border bg-surface p-6 shadow-sm">
      <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-accent text-primary transition-transform duration-300 group-hover:scale-110">
        <Icon className="h-5 w-5" />
      </div>
      <h3 className="text-base font-semibold text-foreground">{title}</h3>
      <p className="mt-2 text-sm leading-relaxed text-secondary">{body}</p>
    </div>
  );
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div>
      <div className="text-2xl font-semibold tracking-tight text-foreground tabular">{value}</div>
      <div className="mt-1 text-xs text-muted">{label}</div>
    </div>
  );
}

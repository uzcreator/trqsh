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
  Download,
  Play,
  Share2,
  SearchCode,
} from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Terminal } from "@/components/terminal";
import { Reveal } from "@/components/reveal";
import { InstallTabs } from "@/components/install-tabs";
import { LatencyChart } from "@/components/latency-chart";
import { ComparisonTable } from "@/components/comparison-table";
import { HeroVisual } from "@/components/hero-visual";
import { Tilt } from "@/components/tilt";
import { SmartDownload } from "@/components/smart-download";
import { ScrollStack, StackPanel } from "@/components/scroll-stack";
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
  { icon: TerminalIcon, title: "Scriptable CLI", body: "`trqsh http 3000`, config files, and exit codes for automation." },
];

const STEPS = [
  {
    icon: Download,
    kicker: "Step 01",
    title: "Install in one line",
    body: "Grab the desktop app for your OS, or install the open-source CLI with your package manager. No account required to try it.",
    code: "brew install trqsh/tap/trqsh",
  },
  {
    icon: Play,
    kicker: "Step 02",
    title: "Run one command",
    body: "Point trqsh at a local port. It opens a QUIC session to the nearest edge and binds a public route — TLS and all.",
    code: "trqsh http 3000",
  },
  {
    icon: Share2,
    kicker: "Step 03",
    title: "Share your live URL",
    body: "Your localhost is now on the internet at a real HTTPS address you can send to anyone, any device, anywhere.",
    code: "https://tidy-otter-4f2a.trqsh.uz",
  },
  {
    icon: SearchCode,
    kicker: "Step 04",
    title: "Inspect & replay",
    body: "Watch every request live at localhost:4040, inspect headers and bodies, and replay any call while you debug.",
    code: "http://localhost:4040",
  },
];

const TRUST = [
  "Open-source agent",
  "QUIC / HTTP-3 transport",
  "HTTP · TCP · UDP",
  "macOS · Windows · Linux",
  "Custom domains & teams",
  "Reserved subdomains",
  "Request inspector & replay",
];

export default function LandingPage() {
  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="tunnel-grid" aria-hidden />
        <div className="aurora" aria-hidden />
        <div className="relative mx-auto max-w-content px-4 pb-20 pt-14 sm:px-6 sm:pt-20">
          <div className="grid items-center gap-12 lg:grid-cols-2">
            <div className="animate-fade-up">
              <Badge variant="default" className="mb-5 border border-brand/20">
                <Signal className="h-3.5 w-3.5" /> Now running on QUIC / HTTP-3
              </Badge>
              <h1 className="text-4xl font-semibold leading-[1.05] tracking-tight text-foreground sm:text-5xl xl:text-6xl">
                Your localhost,
                <br />
                <span className="gradient-text">live on the internet.</span>
              </h1>
              <p className="mt-5 max-w-xl text-lg text-secondary">
                trqsh exposes a local port to a public HTTPS URL in seconds — over QUIC for lower
                latency, with UDP support, a desktop app, and a free tier that stays out of your way.
              </p>

              <div className="mt-7">
                <SmartDownload variant="hero" />
              </div>

              <div className="mt-8 max-w-md">
                <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted">
                  Or install the CLI
                </p>
                <InstallTabs />
              </div>
            </div>

            <div className="animate-fade-up lg:pl-4" style={{ animationDelay: "120ms" }}>
              <div className="relative mx-auto w-full max-w-xl">
                <div className="pointer-events-none absolute -inset-x-12 -inset-y-16 -z-10" aria-hidden>
                  <HeroVisual className="h-full w-full" />
                </div>
                <Tilt glare className="rounded-xl">
                  <div className="tilt-layer" style={{ ["--z" as string]: "40px" }}>
                    <Terminal />
                  </div>
                </Tilt>
              </div>
              <p className="mt-4 text-center text-sm text-muted">
                One command. A live URL, TLS, and an inspector — no signup required to try.
              </p>
            </div>
          </div>
        </div>

        {/* Trust marquee */}
        <div className="border-y border-border bg-surface/40 py-4 backdrop-blur-sm">
          <div className="marquee-mask mx-auto max-w-content overflow-hidden px-4 sm:px-6">
            <div className="marquee-track gap-8 text-sm text-muted">
              {[...TRUST, ...TRUST].map((t, i) => (
                <span key={i} className="flex items-center gap-8 whitespace-nowrap">
                  {t}
                  <span className="text-brand/50">◆</span>
                </span>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* How it works — scroll-stacking sections */}
      <section className="mx-auto max-w-content px-4 py-24 sm:px-6">
        <Reveal className="mx-auto mb-4 max-w-2xl text-center">
          <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">How it works</p>
          <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            From localhost to live in four moves
          </h2>
          <p className="mt-3 text-secondary">
            Scroll — each step stacks over the last, exactly how a request flows through trqsh.
          </p>
        </Reveal>

        <ScrollStack className="mt-10">
          {STEPS.map((step, i) => (
            <StackPanel key={step.title} index={i}>
              <div className="tunnel-grid opacity-40" aria-hidden />
              <div className="relative grid items-center gap-8 p-8 sm:grid-cols-2 sm:p-12">
                <div>
                  <div className="mb-5 inline-flex h-12 w-12 items-center justify-center rounded-xl bg-accent text-brand">
                    <step.icon className="h-6 w-6" />
                  </div>
                  <p className="text-xs font-semibold uppercase tracking-widest text-muted">
                    {step.kicker}
                  </p>
                  <h3 className="mt-1 text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
                    {step.title}
                  </h3>
                  <p className="mt-3 max-w-md text-secondary">{step.body}</p>
                </div>
                <div className="relative">
                  <div className="glass overflow-hidden rounded-xl">
                    <div className="flex items-center gap-2 border-b border-white/5 px-4 py-2.5">
                      <span className="h-2.5 w-2.5 rounded-full bg-brand/70" />
                      <span className="text-xs text-muted">trqsh</span>
                    </div>
                    <pre className="overflow-x-auto px-5 py-6 font-mono text-sm text-foreground">
                      <span className="text-muted">$ </span>
                      {step.code}
                    </pre>
                  </div>
                  <span
                    className="absolute -right-3 -top-3 grid h-9 w-9 place-items-center rounded-full bg-brand text-sm font-bold text-primary-foreground shadow-lg"
                    aria-hidden
                  >
                    {i + 1}
                  </span>
                </div>
              </div>
            </StackPanel>
          ))}
        </ScrollStack>
      </section>

      {/* Differentiators */}
      <Section
        eyebrow="Why trqsh"
        title="Built to be the fastest, friendliest way to share localhost"
        subtitle="Every feature protects one of five promises: speed, a generous free tier, a great GUI, every protocol, and an open agent."
      >
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {DIFFERENTIATORS.map((f, i) => (
            <Reveal key={f.title} delay={(i % 3) * 90} className="h-full">
              <Tilt max={7} className="h-full">
                <FeatureCard icon={f.icon} title={f.title} body={f.body} />
              </Tilt>
            </Reveal>
          ))}
        </div>
      </Section>

      {/* Speed / QUIC */}
      <section className="relative overflow-hidden border-y border-border bg-surface/40">
        <div className="aurora opacity-30" aria-hidden />
        <div className="relative mx-auto max-w-content px-4 py-24 sm:px-6">
          <div className="grid items-center gap-12 lg:grid-cols-2">
            <div>
              <Badge variant="default" className="mb-4 border border-brand/20">
                <Gauge className="h-3.5 w-3.5" /> Speed
              </Badge>
              <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
                QUIC keeps you fast where TCP falls apart
              </h2>
              <p className="mt-4 text-secondary">
                A dropped packet on a TCP tunnel stalls <em>everything</em> behind it — head-of-line
                blocking. QUIC carries each stream independently, so one lost packet doesn&apos;t
                freeze the rest. On a flaky café Wi-Fi or a train, that&apos;s the difference between
                snappy and unusable.
              </p>
              <ul className="mt-5 flex flex-col gap-2 text-sm text-secondary">
                {[
                  "Independent streams — no head-of-line blocking",
                  "Connection migration across network changes",
                  "Automatic TCP + yamux fallback where UDP is blocked",
                ].map((t) => (
                  <li key={t} className="flex items-center gap-2">
                    <span className="h-1.5 w-1.5 rounded-full bg-brand" /> {t}
                  </li>
                ))}
              </ul>
            </div>
            <Reveal>
              <Tilt max={5} glare className="rounded-lg">
                <LatencyChart />
              </Tilt>
            </Reveal>
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
              <div className="border-gradient card-hover flex h-full gap-3 rounded-lg border border-border bg-surface p-5">
                <f.icon className="h-5 w-5 shrink-0 text-brand" />
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
      <section className="border-y border-border bg-surface/40">
        <div className="mx-auto max-w-content px-4 py-24 sm:px-6">
          <div className="mx-auto mb-10 max-w-2xl text-center">
            <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">Honest comparison</p>
            <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
              How trqsh stacks up
            </h2>
            <p className="mt-3 text-secondary">
              We&apos;ll give credit where it&apos;s due — and show you exactly where trqsh pulls ahead.
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
        <div className="mx-auto flex max-w-3xl flex-col items-center gap-6 rounded-2xl border border-border bg-surface/70 p-8 text-center backdrop-blur-sm glow-brand">
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
      <section className="relative overflow-hidden border-t border-border">
        <div className="aurora opacity-40" aria-hidden />
        <div className="relative mx-auto max-w-content px-4 py-24 text-center sm:px-6">
          <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            Ship your localhost in 60 seconds
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-secondary">
            The website already picked the right build for your machine. Grab it and run one command.
          </p>
          <div className="mx-auto mt-8 max-w-3xl text-left">
            <SmartDownload variant="card" />
          </div>
          <div className="mt-6">
            <Link href="/docs/quickstart" className={cn(buttonVariants({ variant: "ghost", size: "lg" }))}>
              Read the quickstart <ArrowRight className="h-4 w-4" />
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
    <section className="mx-auto max-w-content px-4 py-24 sm:px-6">
      <Reveal className="mx-auto mb-10 max-w-2xl text-center">
        <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">{eyebrow}</p>
        <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">{title}</h2>
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
    <div className="border-gradient group h-full rounded-xl border border-border bg-surface p-6 shadow-sm transition-shadow duration-300 hover:shadow-[0_20px_50px_-25px_rgb(0_0_0/0.8)]">
      <div
        className="tilt-layer mb-4 inline-flex h-11 w-11 items-center justify-center rounded-lg bg-accent text-brand transition-transform duration-300 group-hover:scale-110"
        style={{ ["--z" as string]: "26px" }}
      >
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

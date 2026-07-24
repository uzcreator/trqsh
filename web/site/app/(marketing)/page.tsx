import Link from "next/link";
import {
  ArrowRight,
  Gauge,
  Gift,
  GitBranch,
  Globe,
  MonitorSmartphone,
  Radio,
  Download,
  Play,
  Share2,
  SearchCode,
  Signal,
} from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Reveal } from "@/components/reveal";
import { InstallTabs } from "@/components/install-tabs";
import { LatencyChart } from "@/components/latency-chart";
import { ComparisonTable } from "@/components/comparison-table";
import { SmartDownload } from "@/components/smart-download";
import { Tunnel3D } from "@/components/tunnel-3d";
import { signupUrl } from "@/lib/site";
import { planByCode } from "@/lib/plans";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

const free = planByCode("free");

const DIFFERENTIATORS = [
  {
    icon: Gauge,
    title: "QUIC-first, so it's fast",
    body: "Tunnels ride QUIC/HTTP-3 by default — lower latency on lossy and mobile links, with connection migration when you hop Wi-Fi ↔ 5G. Blocked UDP? It falls back to TCP automatically.",
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

const STEPS = [
  { icon: Download, title: "Install in one line", body: "Grab the desktop app or the open-source CLI. No account required to try it.", code: "brew install trqsh/tap/trqsh" },
  { icon: Play, title: "Run one command", body: "Point trqsh at a local port. It opens a QUIC session to the nearest edge.", code: "trqsh http 3000" },
  { icon: Share2, title: "Share your live URL", body: "Your localhost is on the internet at a real HTTPS address, on any device.", code: "https://tidy-otter.trqsh.uz" },
  { icon: SearchCode, title: "Inspect & replay", body: "Watch every request live, inspect headers and bodies, and replay any call.", code: "localhost:4040" },
];

const TRUST = [
  "Open-source agent",
  "QUIC / HTTP-3",
  "HTTP · TCP · UDP",
  "macOS · Windows · Linux",
  "Reserved subdomains",
  "Inspector & replay",
];

export default function LandingPage() {
  return (
    <>
      {/* Hero — immersive 3D tunnel. Background sits at z-0, content at z-10 —
          positive layering (never negative z-index) so WebKit/iOS can't mis-stack
          the scrim over the text. */}
      <section className="relative isolate flex min-h-[78vh] flex-col justify-center overflow-hidden sm:min-h-[88vh]">
        <div className="absolute inset-0 z-0" aria-hidden>
          <Tunnel3D className="pointer-events-none absolute inset-0" />
          <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-page/70 via-page/40 to-page sm:bg-gradient-to-r sm:from-page sm:via-page/65 sm:to-transparent" />
          <div className="pointer-events-none absolute inset-x-0 top-0 h-20 bg-gradient-to-b from-page to-transparent" />
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-gradient-to-t from-page to-transparent" />
        </div>

        <div className="relative z-10 mx-auto w-full max-w-content px-4 py-16 sm:px-6 sm:py-20">
          <div className="max-w-2xl">
            <div className="animate-fade-up">
              <Badge variant="default" className="mb-5 border border-brand/20">
                <Signal className="h-3.5 w-3.5" /> Now running on QUIC / HTTP-3
              </Badge>
            </div>
            <h1
              className="animate-fade-up text-[2.15rem] font-semibold leading-[1.06] tracking-tight text-foreground sm:text-5xl xl:text-6xl"
              style={{ animationDelay: "90ms" }}
            >
              Your localhost,
              <br />
              <span className="gradient-text">live on the internet.</span>
            </h1>
            <p
              className="animate-fade-up mt-5 max-w-xl text-base leading-relaxed text-secondary sm:text-lg"
              style={{ animationDelay: "180ms" }}
            >
              trqsh exposes a local port to a public HTTPS URL in seconds — over QUIC for lower
              latency, with UDP support, a desktop app, and a free tier that stays out of your way.
            </p>

            <div className="animate-fade-up mt-8" style={{ animationDelay: "270ms" }}>
              <SmartDownload variant="hero" />
            </div>

            <div className="animate-fade-up mt-8 max-w-md" style={{ animationDelay: "360ms" }}>
              <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted">Or install the CLI</p>
              <InstallTabs />
            </div>
          </div>
        </div>
      </section>

      {/* Trust row — static */}
      <div className="border-y border-border bg-surface/40">
        <div className="mx-auto flex max-w-content flex-wrap items-center justify-center gap-x-5 gap-y-2 px-4 py-4 text-sm text-muted sm:gap-x-6 sm:px-6">
          {TRUST.map((t, i) => (
            <span key={t} className="flex items-center gap-5 whitespace-nowrap sm:gap-6">
              {t}
              {i < TRUST.length - 1 && <span className="hidden text-brand/40 sm:inline">◆</span>}
            </span>
          ))}
        </div>
      </div>

      {/* How it works — compact, no stacking */}
      <Section
        eyebrow="How it works"
        title="From localhost to live in four moves"
        subtitle="No stacking, no scroll gymnastics — just the four commands that put your machine on the internet."
      >
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {STEPS.map((step, i) => (
            <Reveal key={step.title} delay={(i % 4) * 70} className="h-full">
              <div className="card-hover flex h-full flex-col rounded-xl border border-border bg-surface p-5">
                <div className="mb-4 flex items-center justify-between">
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg bg-accent text-brand">
                    <step.icon className="h-5 w-5" />
                  </span>
                  <span className="font-mono text-xs font-semibold text-muted tabular">0{i + 1}</span>
                </div>
                <h3 className="text-base font-semibold text-foreground">{step.title}</h3>
                <p className="mt-1.5 flex-1 text-sm leading-relaxed text-secondary">{step.body}</p>
                <code className="mt-4 block overflow-x-auto whitespace-nowrap rounded-md border border-border bg-page px-3 py-2 font-mono text-xs text-brand">
                  {step.code}
                </code>
              </div>
            </Reveal>
          ))}
        </div>
      </Section>

      {/* Differentiators */}
      <Section
        eyebrow="Why trqsh"
        title="The fastest, friendliest way to share localhost"
        subtitle="Five promises: speed, a generous free tier, a great app, every protocol, and an open agent."
      >
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {DIFFERENTIATORS.map((f, i) => (
            <Reveal key={f.title} delay={(i % 3) * 80} className="h-full">
              <FeatureCard icon={f.icon} title={f.title} body={f.body} />
            </Reveal>
          ))}
        </div>
      </Section>

      {/* Speed / QUIC */}
      <section className="relative overflow-hidden border-y border-border bg-surface/40">
        <div className="mx-auto max-w-content px-4 py-20 sm:px-6">
          <div className="grid items-center gap-12 lg:grid-cols-2">
            <Reveal>
              <Badge variant="default" className="mb-4 border border-brand/20">
                <Gauge className="h-3.5 w-3.5" /> Speed
              </Badge>
              <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
                QUIC keeps you fast where TCP falls apart
              </h2>
              <p className="mt-4 text-secondary">
                A dropped packet on a TCP tunnel stalls <em>everything</em> behind it — head-of-line
                blocking. QUIC carries each stream independently, so one lost packet doesn&apos;t
                freeze the rest. On flaky café Wi-Fi or a train, that&apos;s the difference between
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
            </Reveal>
            <Reveal delay={80}>
              <LatencyChart />
            </Reveal>
          </div>
        </div>
      </section>

      {/* Comparison */}
      <section className="border-b border-border">
        <div className="mx-auto max-w-content px-4 py-20 sm:px-6">
          <Reveal className="mx-auto mb-10 max-w-2xl text-center">
            <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">Honest comparison</p>
            <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">How trqsh stacks up</h2>
            <p className="mt-3 text-secondary">
              Credit where it&apos;s due — and exactly where trqsh pulls ahead.
            </p>
          </Reveal>
          <ComparisonTable />
        </div>
      </section>

      {/* Pricing + final CTA, merged */}
      <section className="relative overflow-hidden">
        <div className="hero-glow pointer-events-none absolute inset-0 -z-10" aria-hidden />
        <div className="mx-auto max-w-content px-4 py-24 text-center sm:px-6">
          <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">Start free</p>
          <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            Ship your localhost in 60 seconds
          </h2>
          <p className="mx-auto mt-3 max-w-xl text-secondary">
            The same numbers our edge actually enforces — free forever, upgrade only when you outgrow it.
          </p>

          {free && (
            <div className="mx-auto mt-8 grid max-w-2xl grid-cols-2 gap-4 sm:grid-cols-4">
              <Stat value={formatBytes(free.max_bandwidth_bytes_mo)} label="Free / mo" />
              <Stat value={String(free.max_concurrent_tunnels)} label="Concurrent tunnels" />
              <Stat value={String(free.max_reserved_subdomains)} label="Reserved subdomain" />
              <Stat value="$0" label="Forever" />
            </div>
          )}

          <div className="mx-auto mt-8 max-w-3xl text-left">
            <SmartDownload variant="card" />
          </div>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <a href={signupUrl} className={cn(buttonVariants({ size: "lg" }), "glow-brand")}>
              Start free
            </a>
            <Link href="/pricing" className={cn(buttonVariants({ variant: "outline", size: "lg" }))}>
              See all plans <ArrowRight className="h-4 w-4" />
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
    <div className="border-gradient card-hover group h-full rounded-xl border border-border bg-surface p-6">
      <div className="mb-4 inline-flex h-11 w-11 items-center justify-center rounded-lg bg-accent text-brand transition-transform duration-300 group-hover:scale-110">
        <Icon className="h-5 w-5" />
      </div>
      <h3 className="text-base font-semibold text-foreground">{title}</h3>
      <p className="mt-2 text-sm leading-relaxed text-secondary">{body}</p>
    </div>
  );
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div className="rounded-xl border border-border bg-surface/60 p-4">
      <div className="text-2xl font-semibold tracking-tight text-foreground tabular">{value}</div>
      <div className="mt-1 text-xs text-muted">{label}</div>
    </div>
  );
}

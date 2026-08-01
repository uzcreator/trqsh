import Link from "next/link";
import {
  ArrowRight,
  Gauge,
  Gift,
  GitBranch,
  Info,
  MonitorSmartphone,
  Radio,
  Download,
  Share2,
} from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Reveal } from "@/components/reveal";
import { LatencyChart } from "@/components/latency-chart";
import ThreeDCarousel, { type ThreeDCarouselItem } from "@/components/ThreeDCarousel";
import { SmartDownload } from "@/components/smart-download";
import { Tunnel3D } from "@/components/tunnel-3d";
import PlasmaGlobe from "@/components/PlasmaGlobe";
import { HeroHeadline } from "@/components/hero-headline";
import { signupUrl } from "@/lib/site";
import { planByCode } from "@/lib/plans";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

const free = planByCode("free");

const HERO_INFO = [
  "QUIC/HTTP-3 — lower latency than TCP tunnels.",
  "A generous free tier, plus full UDP support.",
  "Open-source agent with a real desktop app.",
];

// The most essential points from both "how it works" and "why trqsh" — kept
// short deliberately: more cards means tighter angular spacing on the ring,
// which reads as cluttered and makes the auto-rotation feel busy rather than
// smooth. Seven is the sweet spot between "too sparse" and "too crowded".
const ICON_CLASS = "h-5 w-5";

const CAROUSEL_ITEMS: ThreeDCarouselItem[] = [
  {
    icon: <Download className={ICON_CLASS} />,
    title: "Install in one line",
    body: "Grab the desktop app or the open-source CLI. No account required to try it.",
    code: "brew install trqsh/tap/trqsh",
  },
  {
    icon: <Gauge className={ICON_CLASS} />,
    title: "QUIC-first, so it's fast",
    body: "Lower latency on lossy and mobile links, with connection migration when you hop Wi-Fi ↔ 5G.",
    stat: "QUIC / HTTP-3",
  },
  {
    icon: <Gift className={ICON_CLASS} />,
    title: "A free tier that isn't a trap",
    body: free
      ? `${formatBytes(free.max_bandwidth_bytes_mo)} of transfer, free forever — deliberately more generous than ngrok's 2026 tier.`
      : "Generous limits, free forever — deliberately more generous than ngrok's 2026 tier.",
    stat: free ? `${formatBytes(free.max_bandwidth_bytes_mo)} free / mo` : "Free forever",
  },
  {
    icon: <MonitorSmartphone className={ICON_CLASS} />,
    title: "A real desktop app",
    body: "A polished native app for macOS, Windows, and Linux — one-click tunnels, live inspector, replay.",
    stat: "macOS · Windows · Linux",
  },
  {
    icon: <Share2 className={ICON_CLASS} />,
    title: "Share your live URL",
    body: "Your localhost is on the internet at a real HTTPS address, on any device.",
    code: "https://tidy-otter.trqsh.uz",
  },
  {
    icon: <Radio className={ICON_CLASS} />,
    title: "Every protocol, including UDP",
    body: "HTTP, HTTPS, TLS, raw TCP — and UDP, which ngrok simply doesn't do.",
    stat: "HTTP · TCP · UDP",
  },
  {
    icon: <GitBranch className={ICON_CLASS} />,
    title: "Open-source agent",
    body: "Audit it, script it, self-host it — no black box on your machine.",
    stat: "Apache-2.0",
  },
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
          <div className="flex flex-col items-center gap-10 lg:flex-row lg:items-center lg:justify-between lg:gap-8">
            <div className="max-w-2xl">
              <HeroHeadline />

              <HeroInfoChip delayMs={420} />

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
            </div>

            {/* Globe sits beside the copy on wide screens only — see
                components/PlasmaGlobe.tsx for the lg breakpoint gate (avoids
                running two WebGL scenes at once on tablets). */}
            <PlasmaGlobe className="relative hidden h-[380px] w-[380px] shrink-0 lg:block xl:h-[440px] xl:w-[440px]" />
          </div>
        </div>
      </section>

      {/* How it works + Why trqsh, combined into one 3D carousel
          (components/ThreeDCarousel.tsx). Auto-rotates; click a card to bring
          it forward and enlarge it for reading; drag to spin it manually. */}
      <section className="border-y border-border bg-surface/40 py-14 sm:py-16">
        <div className="mx-auto max-w-content px-4 sm:px-6">
          <Reveal className="mx-auto mb-10 max-w-2xl text-center">
            <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">Why trqsh</p>
            <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
              From localhost to live, and why it's worth it
            </h2>
            <p className="mt-3 text-secondary">
              Drag to spin it, or tap a card to bring it forward.
            </p>
          </Reveal>
          <ThreeDCarousel items={CAROUSEL_ITEMS} />
        </div>
      </section>

      {/* Speed / QUIC */}
      <section className="relative overflow-hidden border-y border-border bg-surface/40">
        <div className="mx-auto max-w-content px-4 py-20 sm:px-6">
          <div className="grid grid-cols-1 items-center gap-12 lg:grid-cols-2">
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

      {/* Pricing + final CTA, merged */}
      <section className="relative overflow-hidden">
        <div className="hero-glow pointer-events-none absolute inset-0 -z-10" aria-hidden />
        <div className="mx-auto max-w-content px-4 py-24 text-center sm:px-6">
          <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand">Start free</p>
          <h2 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            Ship your localhost in 60 seconds
          </h2>
          <p className="mx-auto mt-3 max-w-xl text-secondary">
            Free forever, upgrade only when you outgrow it.
          </p>

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

// Small hover/focus affordance next to the hero headline — reveals a
// solid card with a few lines about the site — a plain opaque panel (not
// glassmorphism) so it stays fully legible over the busy hero content.
//
// z-30 on the trigger is load-bearing, not decorative: .animate-fade-up
// animates opacity/transform, and both this chip and the paragraph below it
// use that class. An element with an in-flight (or fill-mode: both, so
// "ever ran") transform/opacity animation gets its own stacking context in
// modern browsers, so the popover's z-20 was only ever being compared
// against its aunt <p> *as a whole promoted layer*, ranked by DOM order —
// the popover lost to the paragraph regardless of its z-index. An explicit
// z-index here forces a real comparison and wins it.
function HeroInfoChip({ delayMs }: { delayMs: number }) {
  return (
    <div
      className="group relative z-30 mt-4 inline-flex w-fit animate-fade-up items-center gap-1.5 text-xs font-medium text-muted"
      style={{ animationDelay: `${delayMs}ms` }}
      tabIndex={0}
    >
      <Info className="h-3.5 w-3.5 text-brand" />
      <span className="border-b border-dashed border-border-strong">What is trqsh, exactly?</span>

      <div className="pointer-events-none absolute left-0 top-full z-20 mt-3 w-80 max-w-[85vw] -translate-y-1 rounded-xl border border-border-strong bg-surface-2 p-3 text-left text-sm normal-case leading-snug text-secondary opacity-0 shadow-2xl transition-all duration-300 ease-out group-hover:translate-y-0 group-hover:opacity-100 group-focus:translate-y-0 group-focus:opacity-100">
        {HERO_INFO.map((line, i) => (
          <p key={line} className={i > 0 ? "mt-1" : undefined}>
            {line}
          </p>
        ))}
      </div>
    </div>
  );
}

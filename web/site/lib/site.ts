// Central site configuration: brand strings, deploy-time URLs, and navigation.
// All external destinations resolve from next.config.mjs `env` (localhost defaults),
// so the same build points at dev or prod by setting env vars — never hardcode.

const dashboardUrl = process.env.RIFT_DASHBOARD_URL || "http://localhost:3000";
const apiUrl = process.env.RIFT_API_URL || "http://localhost:8080";
const githubRepo = process.env.RIFT_GITHUB_REPO || "rift/rift";
const siteUrl = process.env.RIFT_SITE_URL || "https://rift.dev";
const version = process.env.RIFT_LATEST_VERSION || "0.1.0";

export const site = {
  name: "Rift",
  tagline: "Your localhost, live on the internet.",
  description:
    "Rift is a developer tunneling service that exposes localhost to the public internet over QUIC — lower latency on lossy networks, a genuinely generous free tier, UDP support, and a first-class desktop app.",
  version,
  siteUrl,
  dashboardUrl,
  apiUrl,
  githubRepo,
  githubUrl: `https://github.com/${githubRepo}`,
  // Public wildcard where tunnels appear (Part 00 §1); shown in examples.
  tunnelDomain: "rift.sh",
  // curl|sh installer served from the marketing origin (Part 08 release/install.sh).
  installShUrl: `${siteUrl}/install.sh`,
} as const;

/** Where "Start free" / "Sign up" go — the dashboard's email+OAuth entry (Part 06/05). */
export const signupUrl = `${dashboardUrl}/login`;
export const loginUrl = `${dashboardUrl}/login`;
/** OAuth deep-links straight into the control API (Part 05). */
export const oauthUrl = (provider: "github" | "google") => `${apiUrl}/v1/auth/oauth/${provider}`;

export const primaryNav = [
  { label: "Pricing", href: "/pricing" },
  { label: "Download", href: "/download" },
  { label: "Docs", href: "/docs" },
] as const;

export const footerNav: { title: string; links: { label: string; href: string; external?: boolean }[] }[] = [
  {
    title: "Product",
    links: [
      { label: "Pricing", href: "/pricing" },
      { label: "Download", href: "/download" },
      { label: "Quickstart", href: "/docs/quickstart" },
      { label: "Changelog", href: `${site.githubUrl}/releases`, external: true },
    ],
  },
  {
    title: "Docs",
    links: [
      { label: "Documentation", href: "/docs" },
      { label: "API reference", href: "/docs/api" },
      { label: "Configuration", href: "/docs/configuration" },
      { label: "Troubleshooting", href: "/docs/errors" },
    ],
  },
  {
    title: "Developers",
    links: [
      { label: "Open-source agent", href: site.githubUrl, external: true },
      { label: "Self-hosting", href: "/docs/self-hosting" },
      { label: "Custom domains", href: "/docs/custom-domains" },
      { label: "Security", href: "/docs/security" },
    ],
  },
  {
    title: "Company",
    links: [
      { label: "Sign in", href: loginUrl, external: true },
      { label: "Start free", href: signupUrl, external: true },
      { label: "Terms", href: "/terms" },
      { label: "Privacy", href: "/privacy" },
    ],
  },
];

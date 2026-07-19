/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Public deploy-time configuration. These are inlined at build so both Server
  // and Client Components can read them. All have localhost-friendly defaults so
  // the site runs against a local stack out of the box.
  env: {
    // Where "Sign up" / "Dashboard" / "Log in" CTAs point (Part 06).
    RIFT_DASHBOARD_URL: process.env.RIFT_DASHBOARD_URL || "http://localhost:3000",
    // Control API (Part 05) — used for the OAuth deep-links on signup.
    RIFT_API_URL: process.env.RIFT_API_URL || "http://localhost:8080",
    // GitHub repo that hosts releases (Part 08 goreleaser output).
    RIFT_GITHUB_REPO: process.env.RIFT_GITHUB_REPO || "rift/rift",
    // Canonical public origin (sitemap, robots, Open Graph, docs links).
    RIFT_SITE_URL: process.env.RIFT_SITE_URL || "https://rift.dev",
    // Latest published version — drives download filenames (matches goreleaser
    // `rift_<version>_<os>_<arch>` and the GUI installer names).
    RIFT_LATEST_VERSION: process.env.RIFT_LATEST_VERSION || "0.1.0",
  },
};

export default nextConfig;

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Self-contained server build for Docker (.next/standalone/server.js).
  output: "standalone",
  // Public deploy-time configuration. These are inlined at build so both Server
  // and Client Components can read them. All have localhost-friendly defaults so
  // the site runs against a local stack out of the box.
  env: {
    // Where "Sign up" / "Dashboard" / "Log in" CTAs point (Part 06).
    TRQSH_DASHBOARD_URL: process.env.TRQSH_DASHBOARD_URL || "https://app.trqsh.uz",
    // Control API (Part 05) — used for the OAuth deep-links on signup.
    TRQSH_API_URL: process.env.TRQSH_API_URL || "https://api.trqsh.uz",
    // GitHub repo that hosts releases (Part 08 goreleaser output).
    TRQSH_GITHUB_REPO: process.env.TRQSH_GITHUB_REPO || "trqsh-uz/trqsh",
    // Canonical public origin (sitemap, robots, Open Graph, docs links).
    TRQSH_SITE_URL: process.env.TRQSH_SITE_URL || "https://trqsh.uz",
    // Latest published version — drives download filenames (matches goreleaser
    // `trqsh_<version>_<os>_<arch>` and the GUI installer names).
    TRQSH_LATEST_VERSION: process.env.TRQSH_LATEST_VERSION || "0.1.0",
  },
};

export default nextConfig;

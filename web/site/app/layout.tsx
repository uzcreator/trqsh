import type { Metadata } from "next";
import "./globals.css";
import { site } from "@/lib/site";
import { SiteHeader } from "@/components/site-header";

export const metadata: Metadata = {
  metadataBase: new URL(site.siteUrl),
  title: {
    default: `${site.name} — ${site.tagline}`,
    template: `%s · ${site.name}`,
  },
  description: site.description,
  applicationName: site.name,
  keywords: [
    "tunnel",
    "localhost",
    "ngrok alternative",
    "cloudflare tunnel alternative",
    "QUIC",
    "HTTP/3",
    "reverse proxy",
    "webhook testing",
    "expose localhost",
  ],
  authors: [{ name: "trqsh" }],
  openGraph: {
    type: "website",
    url: site.siteUrl,
    siteName: site.name,
    title: `${site.name} — ${site.tagline}`,
    description: site.description,
  },
  twitter: {
    card: "summary_large_image",
    title: `${site.name} — ${site.tagline}`,
    description: site.description,
  },
  robots: { index: true, follow: true },
};

export const viewport = {
  themeColor: "#060908",
  colorScheme: "dark",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  // trqsh renders as a committed dark experience (black + dark-green + dark-blue),
  // so the theme is fixed at the root — no toggle, no flash, fully static.
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body className="min-h-screen antialiased">
        <SiteHeader />
        {children}
      </body>
    </html>
  );
}

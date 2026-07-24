import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "trqsh Dashboard",
  description: "Manage your trqsh tunnels, domains, API keys, usage, and billing.",
};

// The dashboard commits to the trqsh dark theme (matches the marketing site).
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body className="min-h-screen">{children}</body>
    </html>
  );
}

import localFont from "next/font/local";

// Self-hosted variable/static fonts (latin subset), committed under app/fonts so
// the production Docker build never reaches out to Google Fonts at build time —
// the type system is deterministic and fully self-hosted. CSS variables are
// consumed by tailwind.config.ts (fontFamily) and globals.css (body + headings):
//
//   --font-display  Instrument Serif  large statement headings + hero + wordmark
//   --font-sans     Onest             body + UI + small headings (quiet, legible)
//   --font-mono     JetBrains Mono    terminal, code, install tabs
//
// next/font statically analyses this call, so every option must be an inline
// literal (no shared consts / spreads).
export const fontDisplay = localFont({
  // Instrument Serif ships a single 400 master; declaring a full weight *range*
  // makes the browser render that master for any requested weight instead of
  // synthesising an ugly faux-bold on a high-contrast serif.
  src: "./fonts/instrument-serif.woff2",
  weight: "100 900",
  style: "normal",
  variable: "--font-display",
  display: "swap",
  fallback: ["Georgia", "Times New Roman", "serif"],
});

export const fontSans = localFont({
  src: "./fonts/onest-wght.woff2",
  weight: "100 900",
  style: "normal",
  variable: "--font-sans",
  display: "swap",
  fallback: ["system-ui", "-apple-system", "Segoe UI", "Roboto", "sans-serif"],
});

export const fontMono = localFont({
  src: "./fonts/jetbrains-mono-wght.woff2",
  weight: "100 800",
  style: "normal",
  variable: "--font-mono",
  display: "swap",
  fallback: ["SFMono-Regular", "Consolas", "Liberation Mono", "Menlo", "monospace"],
});

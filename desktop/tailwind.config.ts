import type { Config } from "tailwindcss";

// Desktop's own "operator console" design tokens — RGB-channel CSS variables
// in src/index.css. Independent from the other trqsh surfaces by design.
const rgb = (v: string) => `rgb(var(${v}) / <alpha-value>)`;

const config: Config = {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        page: rgb("--page"),
        surface: rgb("--surface"),
        "surface-2": rgb("--surface-2"),
        card: rgb("--surface"),
        foreground: rgb("--foreground"),
        secondary: rgb("--secondary-ink"),
        muted: rgb("--muted-ink"),
        border: rgb("--border"),
        "border-strong": rgb("--border-strong"),
        primary: {
          DEFAULT: rgb("--primary"),
          foreground: rgb("--primary-foreground"),
        },
        accent: rgb("--accent"),
        ring: rgb("--ring"),
        // Reserved for data values/links (URLs, byte counts) — keeps numbers
        // visually distinct from primary actions and chrome.
        wire: rgb("--wire"),
        good: rgb("--good"),
        warning: rgb("--warning"),
        serious: rgb("--serious"),
        critical: rgb("--critical"),
        "series-1": rgb("--series-1"),
        "series-2": rgb("--series-2"),
      },
      borderColor: { DEFAULT: rgb("--border") },
      // A deliberate 3-tier radius system (Tailwind's own `xl` default happened
      // to equal our `lg`, erasing the distinction below — now explicit):
      // `lg` = docked/persistent panels (console-panel, channel strips, status
      // bar); `xl` = transient floating overlays (Dialog, CommandPalette,
      // terminal panel); `md` = interactive controls (buttons, inputs, tabs).
      // `full` (pills/indicators/switch) stays Tailwind's default, unlisted.
      borderRadius: { lg: "0.75rem", xl: "1rem", md: "0.5rem", sm: "0.3rem" },
      fontFamily: {
        sans: ["system-ui", "-apple-system", '"Segoe UI"', "Roboto", "sans-serif"],
        // Data/URL text and the embedded terminal share this stack so they read
        // as one instrument voice, not the browser's undeclared mono default.
        mono: [
          "ui-monospace",
          '"Cascadia Code"',
          '"SF Mono"',
          "Consolas",
          '"JetBrains Mono"',
          "monospace",
        ],
      },
    },
  },
  plugins: [],
};

export default config;

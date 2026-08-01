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
      borderRadius: { lg: "0.75rem", md: "0.5rem", sm: "0.3rem" },
      fontFamily: {
        sans: ["system-ui", "-apple-system", '"Segoe UI"', "Roboto", "sans-serif"],
      },
    },
  },
  plugins: [],
};

export default config;

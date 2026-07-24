import type { Config } from "tailwindcss";

// Brand tokens are CSS variables (RGB channels) in app/globals.css — kept
// identical to the marketing site (web/site) so both surfaces look like one
// product (trqsh dark green/blue brand).
const rgb = (v: string) => `rgb(var(${v}) / <alpha-value>)`;

const config: Config = {
  darkMode: "class",
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        page: rgb("--page"),
        surface: rgb("--surface"),
        "surface-2": rgb("--surface-2"),
        card: rgb("--surface"),
        foreground: rgb("--foreground"),
        brand: rgb("--brand"),
        "brand-2": rgb("--brand-2"),
        "brand-3": rgb("--brand-3"),
        glow: rgb("--glow"),
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
        good: rgb("--good"),
        warning: rgb("--warning"),
        serious: rgb("--serious"),
        critical: rgb("--critical"),
        "series-1": rgb("--series-1"),
        "series-2": rgb("--series-2"),
        "series-3": rgb("--series-3"),
        "series-4": rgb("--series-4"),
        "series-5": rgb("--series-5"),
        grid: rgb("--grid"),
        baseline: rgb("--baseline"),
      },
      borderColor: {
        DEFAULT: rgb("--border"),
      },
      borderRadius: {
        lg: "0.6rem",
        md: "0.45rem",
        sm: "0.3rem",
      },
      fontFamily: {
        sans: ["system-ui", "-apple-system", '"Segoe UI"', "Roboto", "sans-serif"],
        mono: ['"SFMono-Regular"', "Consolas", '"Liberation Mono"', "Menlo", "monospace"],
      },
      maxWidth: {
        content: "72rem",
      },
      keyframes: {
        "fade-up": {
          from: { opacity: "0", transform: "translateY(12px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "fade-in": {
          from: { opacity: "0" },
          to: { opacity: "1" },
        },
        "scale-in": {
          from: { opacity: "0", transform: "scale(0.96)" },
          to: { opacity: "1", transform: "scale(1)" },
        },
        shimmer: {
          "100%": { transform: "translateX(100%)" },
        },
        "pulse-ring": {
          "0%": { boxShadow: "0 0 0 0 rgb(var(--good) / 0.5)" },
          "70%": { boxShadow: "0 0 0 6px rgb(var(--good) / 0)" },
          "100%": { boxShadow: "0 0 0 0 rgb(var(--good) / 0)" },
        },
        "slide-down": {
          from: { opacity: "0", transform: "translateY(-12px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "slide-in-left": {
          from: { opacity: "0", transform: "translateX(-100%)" },
          to: { opacity: "1", transform: "translateX(0)" },
        },
      },
      animation: {
        "fade-up": "fade-up 0.5s cubic-bezier(0.22,1,0.36,1) both",
        "fade-in": "fade-in 0.4s ease-out both",
        "scale-in": "scale-in 0.4s cubic-bezier(0.22,1,0.36,1) both",
        shimmer: "shimmer 2s infinite",
        "pulse-ring": "pulse-ring 2s ease-out infinite",
        "slide-down": "slide-down 0.4s cubic-bezier(0.22,1,0.36,1) both",
        "slide-in-left": "slide-in-left 0.25s cubic-bezier(0.22,1,0.36,1) both",
      },
    },
  },
  plugins: [],
};

export default config;

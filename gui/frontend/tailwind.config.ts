import type { Config } from "tailwindcss";

// Design tokens shared with web/dashboard (Part 06) and web/site (Part 09):
// RGB-channel CSS variables in src/index.css, dataviz reference palette. Keeping
// this identical across the three surfaces is what makes the brand consistent.
const rgb = (v: string) => `rgb(var(${v}) / <alpha-value>)`;

const config: Config = {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        page: rgb("--page"),
        surface: rgb("--surface"),
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
        good: rgb("--good"),
        warning: rgb("--warning"),
        serious: rgb("--serious"),
        critical: rgb("--critical"),
        "series-1": rgb("--series-1"),
        "series-2": rgb("--series-2"),
      },
      borderColor: { DEFAULT: rgb("--border") },
      borderRadius: { lg: "0.6rem", md: "0.45rem", sm: "0.3rem" },
      fontFamily: {
        sans: ["system-ui", "-apple-system", '"Segoe UI"', "Roboto", "sans-serif"],
      },
    },
  },
  plugins: [],
};

export default config;

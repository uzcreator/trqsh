import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Lock the WebView down with a strict Content-Security-Policy. The app is fully
// self-contained (embedded assets, no remote scripts/styles/fonts, and the
// Wails bridge is a native postMessage channel — not HTTP), so we can forbid
// everything off-origin. This is defense-in-depth against any injected content
// in the embedded browser. Injected at BUILD time only so `vite dev` HMR
// (inline scripts + websocket) keeps working during development.
const CSP = [
  "default-src 'self'",
  "base-uri 'none'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "form-action 'none'",
  "img-src 'self' data:",
  "font-src 'self' data:",
  "style-src 'self' 'unsafe-inline'",
  "script-src 'self'",
  "connect-src 'self'",
].join("; ");

function cspPlugin(): Plugin {
  return {
    name: "trqsh-csp",
    apply: "build",
    transformIndexHtml() {
      return [
        {
          tag: "meta",
          attrs: { "http-equiv": "Content-Security-Policy", content: CSP },
          injectTo: "head-prepend",
        },
      ];
    },
  };
}

// Vite builds the GUI frontend; Wails serves this build (or proxies the dev
// server) inside the native window.
export default defineConfig({
  plugins: [react(), cspPlugin()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  base: "./",
  build: { outDir: "dist", emptyOutDir: true },
  server: { port: 9245, strictPort: false },
});

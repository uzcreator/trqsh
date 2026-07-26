import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Frontend for the Tauri shell. Tauri serves this build inside the native
// WebView (or proxies the dev server). The Content-Security-Policy is managed by
// Tauri (src-tauri/tauri.conf.json → app.security.csp) so it can allow the
// loopback control API origin; we don't inject a build-time meta CSP here.
//
// The dev server runs on a fixed port so Tauri's devUrl is stable, and on
// 127.0.0.1 so the control API's CORS origin reflection lines up during browser
// dev against a locally-run `trqsh daemon`.
const host = process.env.TAURI_DEV_HOST;

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  // Tauri expects a relative base so assets resolve under the app protocol.
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    // Tauri uses a modern WebView; target its engines for smaller output.
    target: ["es2021", "chrome105", "safari15"],
  },
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
    host: host || "localhost",
    hmr: host ? { protocol: "ws", host, port: 1421 } : undefined,
    watch: { ignored: ["**/src-tauri/**"] },
  },
});

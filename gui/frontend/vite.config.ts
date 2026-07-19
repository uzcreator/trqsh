import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Vite builds the GUI frontend; Wails serves this build (or proxies the dev
// server) inside the native window.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  base: "./",
  build: { outDir: "dist", emptyOutDir: true },
  server: { port: 9245, strictPort: false },
});

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  build: {
    // Embed target — Go side reads from internal/webdist/dist/
    outDir: path.resolve(__dirname, "../internal/webdist/dist"),
    emptyOutDir: true,
    // No source maps in the embedded production bundle — keeps binary size
    // down and avoids serving original TypeScript through the SPA. Set to
    // 'hidden' or true if you need them locally, but rebuild before
    // committing so dist/ stays clean.
    sourcemap: false,
    chunkSizeWarningLimit: 1500, // AntD Pro is chonky; expected
  },
  server: {
    port: 5173,
    proxy: {
      "/api":     "http://127.0.0.1:8765",
      "/healthz": "http://127.0.0.1:8765",
      "/readyz":  "http://127.0.0.1:8765",
    },
  },
});

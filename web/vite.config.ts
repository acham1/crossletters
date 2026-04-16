import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Vite config:
// - dev server proxies /api to the local Go backend so cookies and fetches work
//   with a single origin
// - build output goes into ../webdist/dist, which the Go binary embeds via
//   //go:embed
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
  build: {
    outDir: "../webdist/dist",
    // Keep the placeholder.txt so `go build` still works on a fresh clone
    // without a frontend build. Vite overwrites index.html and assets anyway.
    emptyOutDir: false,
  },
});

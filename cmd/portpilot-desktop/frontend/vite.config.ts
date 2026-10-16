import { defineConfig } from "vite";

// Wails serves the embedded frontend/dist directory, so assets must use
// relative paths and the output must land in dist.
export default defineConfig({
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    assetsDir: "assets",
    target: "es2020",
  },
  server: {
    port: 5173,
    strictPort: true,
  },
});
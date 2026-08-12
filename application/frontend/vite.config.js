import { defineConfig, loadEnv } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import * as path from "path";
const VITE_ASSET_URL = import.meta.VITE_ASSET_URL || "";
// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  plugins: [
    svelte({
      // runes: true makes legacy syntax a compile error in our own components
      // rather than a silent fallback to legacy mode. Dependencies still ship
      // Svelte 4 source, so they keep compiling in legacy mode.
      dynamicCompileOptions: ({ filename }) =>
        filename.includes("node_modules") ? {} : { runes: true },
    }),
  ],
});

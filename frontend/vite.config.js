import { defineConfig, loadEnv } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import * as path from "path";
const VITE_ASSET_URL = import.meta.VITE_ASSET_URL || "";
// https://vitejs.dev/config/
export default defineConfig({
  base: `${VITE_ASSET_URL}`,

  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  plugins: [svelte({})],
});

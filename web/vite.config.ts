import path from "node:path";
import { fileURLToPath } from "node:url";
import fs from "node:fs/promises";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, type PluginOption } from "vite";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const backendStaticDir = path.resolve(__dirname, "../backend/static");

function syncBackendStaticPlugin(): PluginOption {
  return {
    name: "sync-backend-static",
    apply: "build",
    async closeBundle() {
      if (process.env.SYNC_BACKEND_STATIC !== "true") {
        return;
      }
      const sourceDir = path.resolve(__dirname, "dist");
      await fs.mkdir(path.dirname(backendStaticDir), { recursive: true });
      await fs.rm(backendStaticDir, { recursive: true, force: true });
      await fs.mkdir(backendStaticDir, { recursive: true });
      await fs.cp(sourceDir, backendStaticDir, { recursive: true });
      console.log(`[vite] synced ${sourceDir} -> ${backendStaticDir}`);
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), syncBackendStaticPlugin()],
  server: {
    host: "0.0.0.0",
    port: 5270,
    proxy: {
      "/auth": "http://127.0.0.1:7070",
      "/api": "http://127.0.0.1:7070",
      "/v1/files/image": "http://127.0.0.1:7070",
      "/v1": "http://127.0.0.1:8080",
      "/version": "http://127.0.0.1:7070",
      "/health": "http://127.0.0.1:7070",
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: "dist",
  },
});

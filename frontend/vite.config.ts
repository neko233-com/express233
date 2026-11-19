import babel from "@rolldown/plugin-babel";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import { defineConfig } from "vite-plus";

const backend = process.env.EXPRESS233_DEV_BACKEND || "http://127.0.0.1:23380";

export default defineConfig({
  plugins: [react(), babel({ presets: [reactCompilerPreset()] })],
  server: {
    host: "127.0.0.1",
    proxy: Object.fromEntries(
      ["/api", "/docs", "/healthz", "/readyz", "/metrics"].map((path) => [path, backend]),
    ),
  },
  build: { outDir: "../internal/api/web/dist", emptyOutDir: true },
});

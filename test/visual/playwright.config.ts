import { defineConfig, devices } from "@playwright/test";
import path from "path";

const repoRoot = path.resolve(__dirname, "../..");
const dataDir = path.join(repoRoot, ".visual-e2e-data");
const port = process.env.EXPRESS233_VISUAL_PORT || "39234";
const baseURL = process.env.EXPRESS233_BASE_URL || `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  globalSetup: "./global-setup.ts",
  webServer: {
    command: `go run ${path.join(repoRoot, "cmd/express233-server")} -addr :${port} -data ${dataDir}`,
    url: `${baseURL}/healthz`,
    cwd: repoRoot,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});

import fs from "fs";
import path from "path";

export default async function globalSetup() {
  const repoRoot = path.resolve(__dirname, "../..");
  const tenantDir = path.join(repoRoot, ".visual-e2e-data", "tenants", "default");
  fs.mkdirSync(tenantDir, { recursive: true });
  const yaml = `servers:
  visual-s1:
    replacements:
      game.properties:
        port: "9001"
    post_hook_env:
      SERVER_ID: visual-s1
`;
  fs.writeFileSync(path.join(tenantDir, "server.yaml"), yaml, "utf8");
}

import fs from "fs";
import path from "path";

export default async function globalSetup() {
  const repoRoot = path.resolve(__dirname, "../..");
  const dataDir = process.env.EXPRESS233_VISUAL_DATA_DIR || path.join(repoRoot, ".visual-e2e-data");
  const tenantDir = path.join(dataDir, "userdata", "default");
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

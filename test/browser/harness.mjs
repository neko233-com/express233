import assert from "node:assert/strict";
import { once } from "node:events";
import { spawn, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { setTimeout } from "node:timers/promises";
import { fileURLToPath } from "node:url";

const repo = fileURLToPath(new URL("../../", import.meta.url));
const cli = fileURLToPath(new URL("./node_modules/agent-browser/bin/agent-browser.js", import.meta.url));
const tempRoot = resolve(tmpdir());
const fixturePassword = "root";

async function freePort() {
  const socket = createServer();
  socket.listen(0, "127.0.0.1");
  await once(socket, "listening");
  const { port } = socket.address();
  await new Promise((done, fail) => socket.close((error) => error ? fail(error) : done()));
  return port;
}

async function healthy(base, child) {
  for (let n = 0; n < 100 && child.exitCode === null && !child.killed; n++) {
    try {
      if ((await fetch(base + "/healthz", { signal: AbortSignal.timeout(400) })).ok) return;
    } catch { /* The isolated server is starting. */ }
    await setTimeout(75);
  }
  throw new Error("Isolated Express233 server did not become healthy");
}

export const testID = (name) => `[data-testid="${name}"]`;
export const tab = (name) => `[data-ptab="${name}"]`;
export const text = (selector, expected) => ["wait", "--fn", `document.querySelector(${JSON.stringify(selector)})?.innerText.includes(${JSON.stringify(expected)}) === true`];

export async function withBrowser(verify) {
  const dir = mkdtempSync(join(tempRoot, "express233-browser-"));
  const session = "express233-" + process.pid;
  let child;
  let launched = false;

  function browser(args, input) {
    return spawnSync(process.execPath, [cli, "--session", session, "--config", join(dir, "agent-browser.json"),
      "--allowed-domains", "127.0.0.1", ...args], {
      input, encoding: "utf8", timeout: 120_000, maxBuffer: 8 * 1024 * 1024,
      env: { ...process.env, AGENT_BROWSER_HEADED: "false", AGENT_BROWSER_NO_AUTO_CONNECT: "true", AGENT_BROWSER_DEFAULT_TIMEOUT: "4500" },
    });
  }
  function batch(label, commands) {
    const start = performance.now();
    launched = true;
    const result = browser(["batch", "--json", "--bail", "--omit-command"], JSON.stringify(commands));
    if (result.error) throw result.error;
    let rows;
    try { rows = JSON.parse(result.stdout); } catch {
      throw new Error(`${label}: Agent Browser did not return batch results (${result.stderr.trim()})`);
    }
    assert.ok(Array.isArray(rows), `${label}: ${rows?.error || "expected batch results"}`);
    const failure = rows.find((row) => row.success === false || row.error || row.result?.success === false);
    if (failure || result.status !== 0 || rows.length !== commands.length) {
      const index = failure?.index ?? rows.length - 1;
      // Batch uses omit-command to keep entered values and credentials out of logs.
      throw new Error(`${label}: step ${index + 1} (${commands[index]?.[0] || "batch"}) failed: ${JSON.stringify(failure?.error || failure?.result?.error || result.stderr.trim())}`);
    }
    console.log(`ok  ${label} (${((performance.now() - start) / 1000).toFixed(2)} s)`);
    return rows.map((row) => row.result?.data ?? row.data ?? row.result);
  }

  try {
    writeFileSync(join(dir, "agent-browser.json"), "{}\n");
    mkdirSync(join(dir, "data"));
    const externalBinary = process.env.EXPRESS233_BROWSER_SERVER;
    const binary = externalBinary ? resolve(repo, externalBinary) : join(dir, process.platform === "win32" ? "express233-server.exe" : "express233-server");
    if (!externalBinary) {
      const build = spawnSync("go", ["build", "-o", binary, "./cmd/express233-server"], {
        cwd: repo, env: { ...process.env, CGO_ENABLED: "0" }, stdio: "inherit", timeout: 180_000,
      });
      if (build.status !== 0) throw build.error ?? new Error("Go server build failed");
    }
    const port = await freePort();
    const base = `http://127.0.0.1:${port}`;
    child = spawn(binary, ["serve", "--addr", `127.0.0.1:${port}`, "--data", "data"], {
      cwd: dir, stdio: "ignore", windowsHide: true,
      env: { ...process.env, EXPRESS233_WEB_DIR: "" },
    });
    // Propagate spawn failures as readiness failures, without dumping server environment.
    child.on("error", () => { child.kill(); });
    await healthy(base, child);

    let jwt;
    async function api(path, method = "GET", body, expected = 200) {
      assert.ok(path.startsWith("/api/"), "Fixture writes must remain on the isolated API");
      const headers = jwt ? { Authorization: "Bearer " + jwt } : {};
      if (body !== undefined && !(body instanceof FormData)) headers["Content-Type"] = "application/json";
      const response = await fetch(base + path, {
        method, headers, body: body instanceof FormData ? body : body === undefined ? undefined : JSON.stringify(body),
      });
      assert.equal(response.status, expected, `${method} ${path}`);
      return response.status === 204 ? undefined : response.json();
    }
    const sessionResponse = await api("/api/login", "POST", { username: "root", password: fixturePassword });
    jwt = sessionResponse.token;
    const captureRoot = process.env.EXPRESS233_BROWSER_CAPTURE && resolve(process.env.EXPRESS233_BROWSER_CAPTURE);
    if (captureRoot) mkdirSync(captureRoot, { recursive: true });
    const capture = (name) => captureRoot ? [["screenshot", join(captureRoot, name + ".png")]] : [];
    await verify({ base, batch, api, capture, dir, fixturePassword });
  } finally {
    if (launched) browser(["close"]);
    if (child && child.exitCode === null && child.pid) {
      const stopped = once(child, "close");
      child.kill();
      await Promise.race([stopped, setTimeout(2500)]);
    }
    if (dirname(dir) === tempRoot) rmSync(dir, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
  }
}

import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { normalizeBundleNames } from "./normalize-bundle-names.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const tauriRoot = path.resolve(scriptDir, "..");
const tauriConfigPath = path.join(tauriRoot, "src-tauri", "tauri.conf.json");
const targetRoot = path.join(tauriRoot, "src-tauri", "target");

async function readTauriConfig() {
  const raw = await fs.readFile(tauriConfigPath, "utf8");
  return JSON.parse(raw);
}

function buildCommand() {
  return process.platform === "win32" ? "npx.cmd" : "npx";
}

async function run() {
  const args = ["tauri", "build", ...process.argv.slice(2)];
  const child = spawn(buildCommand(), args, {
    cwd: tauriRoot,
    stdio: "inherit",
  });

  const exitCode = await new Promise((resolve, reject) => {
    child.on("error", reject);
    child.on("close", resolve);
  });

  if (exitCode !== 0) {
    process.exit(exitCode ?? 1);
  }

  const tauriConfig = await readTauriConfig();
  const renamed = await normalizeBundleNames({
    targetRoot,
    productName: tauriConfig.productName,
    version: tauriConfig.version,
    platform: process.platform,
  });

  for (const item of renamed) {
    console.log(`renamed ${path.basename(item.from)} -> ${path.basename(item.to)}`);
  }
}

run().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});

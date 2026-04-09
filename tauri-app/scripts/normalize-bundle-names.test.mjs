import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import {
  buildCanonicalArtifactName,
  detectArtifactArch,
  normalizeBundleNames,
} from "./normalize-bundle-names.mjs";

test("detectArtifactArch normalizes Tauri arch aliases", () => {
  assert.equal(detectArtifactArch("ZimaOS Blue_0.10.39_aarch64.dmg"), "arm64");
  assert.equal(detectArtifactArch("ZimaOS-Blue_0.10.39_x64-setup.exe"), "x64");
});

test("normalizeBundleNames renames macOS dmg outputs to canonical darwin names", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "blue-tauri-macos-"));
  const dmgDir = path.join(tempRoot, "release", "bundle", "dmg");
  await fs.mkdir(dmgDir, { recursive: true });
  await fs.writeFile(path.join(dmgDir, "ZimaOS Blue_0.10.39_aarch64.dmg"), "arm64");
  await fs.writeFile(path.join(dmgDir, "ZimaOS Blue_0.10.39_x64.dmg"), "amd64");

  const renamed = await normalizeBundleNames({
    targetRoot: tempRoot,
    productName: "ZimaOS Blue",
    version: "0.10.39",
    platform: "darwin",
  });

  assert.equal(renamed.length, 2);
  assert.equal(
    await fs.readFile(path.join(dmgDir, "ZimaOS-Blue-0.10.39-darwin-arm64.dmg"), "utf8"),
    "arm64"
  );
  assert.equal(
    await fs.readFile(path.join(dmgDir, "ZimaOS-Blue-0.10.39-darwin-x64.dmg"), "utf8"),
    "amd64"
  );
});

test("normalizeBundleNames renames Windows nsis outputs to canonical windows names", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "blue-tauri-windows-"));
  const nsisDir = path.join(tempRoot, "x86_64-pc-windows-gnu", "release", "bundle", "nsis");
  await fs.mkdir(nsisDir, { recursive: true });
  await fs.writeFile(path.join(nsisDir, "ZimaOS-Blue_0.10.39_x64-setup.exe"), "installer");

  const renamed = await normalizeBundleNames({
    targetRoot: tempRoot,
    productName: "ZimaOS Blue",
    version: "0.10.39",
    platform: "win32",
  });

  assert.equal(renamed.length, 1);
  assert.equal(
    await fs.readFile(path.join(nsisDir, buildCanonicalArtifactName({
      productName: "ZimaOS Blue",
      version: "0.10.39",
      platform: "windows",
      arch: "x64",
      kind: "nsis",
    })), "utf8"),
    "installer"
  );
});

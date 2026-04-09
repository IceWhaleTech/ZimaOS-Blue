import fs from "node:fs/promises";
import path from "node:path";

export function normalizeProductName(productName) {
  return String(productName || "")
    .trim()
    .replace(/[\s_]+/g, "-")
    .replace(/-+/g, "-");
}

export function normalizeArch(arch) {
  const normalized = String(arch || "").trim().toLowerCase();
  if (!normalized) {
    return "";
  }
  if (normalized === "x64" || normalized === "x86_64" || normalized === "amd64") {
    return "amd64";
  }
  if (normalized === "aarch64" || normalized === "arm64") {
    return "arm64";
  }
  return normalized;
}

export function detectArtifactArch(fileName) {
  const match = String(fileName || "").match(/(?:^|[-_])(aarch64|arm64|x64|x86_64|amd64)(?:[-_.]|$)/i);
  return normalizeArch(match?.[1] || "");
}

export function buildCanonicalArtifactName({ productName, version, platform, arch, kind }) {
  const base = `${normalizeProductName(productName)}-${version}-${platform}-${normalizeArch(arch)}`;
  if (kind === "dmg") {
    return `${base}.dmg`;
  }
  if (kind === "nsis") {
    return `${base}-setup.exe`;
  }
  throw new Error(`unsupported artifact kind: ${kind}`);
}

async function directoryExists(dirPath) {
  try {
    const stat = await fs.stat(dirPath);
    return stat.isDirectory();
  } catch {
    return false;
  }
}

async function collectBundleRoots(targetRoot) {
  const roots = [];
  const directBundleRoot = path.join(targetRoot, "release", "bundle");
  if (await directoryExists(directBundleRoot)) {
    roots.push(directBundleRoot);
  }

  let entries = [];
  try {
    entries = await fs.readdir(targetRoot, { withFileTypes: true });
  } catch {
    return roots;
  }

  for (const entry of entries) {
    if (!entry.isDirectory()) {
      continue;
    }
    const bundleRoot = path.join(targetRoot, entry.name, "release", "bundle");
    if (await directoryExists(bundleRoot)) {
      roots.push(bundleRoot);
    }
  }

  return roots;
}

async function renameMatchingArtifacts({ artifactDir, productName, version, platform, kind }) {
  let entries = [];
  try {
    entries = await fs.readdir(artifactDir, { withFileTypes: true });
  } catch {
    return [];
  }

  const renamed = [];
  for (const entry of entries) {
    if (!entry.isFile()) {
      continue;
    }

    const sourceName = entry.name;
    if (!sourceName.includes(version)) {
      continue;
    }

    const arch = detectArtifactArch(sourceName);
    if (!arch) {
      continue;
    }

    const targetName = buildCanonicalArtifactName({
      productName,
      version,
      platform,
      arch,
      kind,
    });

    if (sourceName === targetName) {
      continue;
    }

    const sourcePath = path.join(artifactDir, sourceName);
    const targetPath = path.join(artifactDir, targetName);
    await fs.rm(targetPath, { force: true });
    await fs.rename(sourcePath, targetPath);
    renamed.push({ from: sourcePath, to: targetPath });
  }

  return renamed;
}

export async function normalizeBundleNames({
  targetRoot,
  productName,
  version,
  platform,
}) {
  const bundleRoots = await collectBundleRoots(targetRoot);
  const renamed = [];

  for (const bundleRoot of bundleRoots) {
    if (platform === "darwin") {
      renamed.push(
        ...(await renameMatchingArtifacts({
          artifactDir: path.join(bundleRoot, "dmg"),
          productName,
          version,
          platform: "darwin",
          kind: "dmg",
        }))
      );
    }

    if (platform === "win32") {
      renamed.push(
        ...(await renameMatchingArtifacts({
          artifactDir: path.join(bundleRoot, "nsis"),
          productName,
          version,
          platform: "windows",
          kind: "nsis",
        }))
      );
    }
  }

  return renamed;
}

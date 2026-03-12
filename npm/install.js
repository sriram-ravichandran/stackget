#!/usr/bin/env node
"use strict";

const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const REPO = "sriram-ravichandran/stackget";
const VERSION = require("./package.json").version;
const BIN_DIR = path.join(__dirname, "bin");

function getPlatformInfo() {
  const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };

  const os = platformMap[process.platform];
  const arch = archMap[process.arch];

  if (!os || !arch) {
    throw new Error(
      `Unsupported platform: ${process.platform}/${process.arch}\n` +
      `Install manually from: https://github.com/${REPO}/releases`
    );
  }

  const binName = os === "windows" ? "stackget.exe" : "stackget";
  const archive = `stackget-${os}-${arch}.tar.gz`;
  const downloadUrl = `https://github.com/${REPO}/releases/download/v${VERSION}/${archive}`;
  return { os, arch, binName, archive, downloadUrl };
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    function get(url) {
      const file = fs.createWriteStream(dest);
      https.get(url, { headers: { "User-Agent": "stackget-npm-installer" } }, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          file.close();
          return get(res.headers.location);
        }
        if (res.statusCode !== 200) {
          file.close();
          fs.unlink(dest, () => {});
          return reject(new Error(`HTTP ${res.statusCode} fetching ${url}`));
        }
        res.pipe(file);
        file.on("finish", () => { file.close(); resolve(); });
        file.on("error", (err) => { fs.unlink(dest, () => {}); reject(err); });
      }).on("error", reject);
    }
    get(url);
  });
}

// Recursively find a file by name inside a directory.
function findFile(dir, name) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      const found = findFile(full, name);
      if (found) return found;
    } else if (entry.name === name) {
      return full;
    }
  }
  return null;
}

async function main() {
  const { os, binName, archive, downloadUrl } = getPlatformInfo();

  fs.mkdirSync(BIN_DIR, { recursive: true });

  const binPath = path.join(BIN_DIR, binName);
  if (fs.existsSync(binPath)) return; // already installed

  const tmpArchive = path.join(BIN_DIR, archive);
  const tmpExtract = path.join(BIN_DIR, "_extract");

  console.log(`\nDownloading stackget v${VERSION}...`);
  console.log(`  ${downloadUrl}\n`);

  try {
    await download(downloadUrl, tmpArchive);
  } catch (err) {
    console.error(`\nDownload failed: ${err.message}`);
    console.error(`Install manually: https://github.com/${REPO}/releases`);
    process.exit(1);
  }

  try {
    fs.mkdirSync(tmpExtract, { recursive: true });
    // Extract everything into a temp dir — handles both wrapped and flat archives.
    execSync(`tar -xzf "${tmpArchive}" -C "${tmpExtract}"`, { stdio: "pipe" });

    const found = findFile(tmpExtract, binName);
    if (!found) throw new Error(`${binName} not found in archive`);

    fs.copyFileSync(found, binPath);
    if (os !== "windows") fs.chmodSync(binPath, 0o755);
  } catch (err) {
    console.error(`\nExtraction failed: ${err.message}`);
    process.exit(1);
  } finally {
    try { fs.unlinkSync(tmpArchive); } catch (_) {}
    try { fs.rmSync(tmpExtract, { recursive: true, force: true }); } catch (_) {}
  }

  console.log(`stackget v${VERSION} installed successfully!\n`);
}

main();

#!/usr/bin/env node
/**
 * install.js — postinstall script for the stackget npm package.
 *
 * Downloads the correct pre-built binary from the GitHub release that matches
 * this package version, extracts it into ./bin/, and makes it executable.
 *
 * Supports: macOS (x64 + arm64), Linux (x64 + arm64), Windows (x64 + arm64).
 * Requires: Node ≥ 14, system `tar` (ships with macOS, Linux, and Windows 10+).
 */

"use strict";

const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const REPO = "sriram-ravichandran/stackget";
const VERSION = require("./package.json").version;
const BIN_DIR = path.join(__dirname, "bin");

// ─── Platform detection ───────────────────────────────────────────────────────

function getPlatformInfo() {
  const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };

  const os = platformMap[process.platform];
  const arch = archMap[process.arch];

  if (!os) {
    throw new Error(
      `Unsupported platform: ${process.platform}. ` +
        "Install manually from https://github.com/" + REPO + "/releases"
    );
  }
  if (!arch) {
    throw new Error(
      `Unsupported architecture: ${process.arch}. ` +
        "Install manually from https://github.com/" + REPO + "/releases"
    );
  }

  const binName = os === "windows" ? "stackget.exe" : "stackget";
  const archive = `stackget-${os}-${arch}.tar.gz`;
  const downloadUrl = `https://github.com/${REPO}/releases/download/v${VERSION}/${archive}`;

  return { os, arch, binName, archive, downloadUrl };
}

// ─── Download helper (follows redirects) ─────────────────────────────────────

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);

    function get(url) {
      https
        .get(url, { headers: { "User-Agent": "stackget-npm-installer" } }, (res) => {
          if (res.statusCode === 301 || res.statusCode === 302) {
            file.close();
            // Reopen file for the redirect
            const newFile = fs.createWriteStream(dest);
            get(res.headers.location);
            return;
          }
          if (res.statusCode !== 200) {
            file.close();
            fs.unlink(dest, () => {});
            reject(new Error(`HTTP ${res.statusCode} fetching ${url}`));
            return;
          }
          res.pipe(file);
          file.on("finish", () => {
            file.close();
            resolve();
          });
          file.on("error", (err) => {
            fs.unlink(dest, () => {});
            reject(err);
          });
        })
        .on("error", (err) => {
          fs.unlink(dest, () => {});
          reject(err);
        });
    }

    get(url);
  });
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const { os, arch, binName, archive, downloadUrl } = getPlatformInfo();

  fs.mkdirSync(BIN_DIR, { recursive: true });

  const tmpArchive = path.join(BIN_DIR, archive);
  const binPath = path.join(BIN_DIR, binName);

  // Skip if already installed (e.g. re-running postinstall).
  if (fs.existsSync(binPath)) {
    return;
  }

  console.log(`\nDownloading stackget v${VERSION} for ${os}/${arch}...`);
  console.log(`  ${downloadUrl}\n`);

  try {
    await download(downloadUrl, tmpArchive);
  } catch (err) {
    console.error(`\nFailed to download stackget: ${err.message}`);
    console.error(
      "You can install manually from: https://github.com/" + REPO + "/releases"
    );
    process.exit(1);
  }

  try {
    // tar is available on macOS, all Linux distros, and Windows 10+ (build 17063+).
    execSync(`tar -xzf "${tmpArchive}" -C "${BIN_DIR}" "${binName}"`, {
      stdio: "inherit",
    });
  } catch (err) {
    console.error(`\nFailed to extract archive: ${err.message}`);
    console.error("Ensure 'tar' is available on your PATH.");
    process.exit(1);
  } finally {
    try {
      fs.unlinkSync(tmpArchive);
    } catch (_) {}
  }

  if (os !== "windows") {
    fs.chmodSync(binPath, 0o755);
  }

  console.log(`stackget v${VERSION} installed successfully!\n`);
}

main();

#!/usr/bin/env node
/**
 * run.js — thin wrapper that locates and spawns the stackget binary.
 * This file is the `bin.stackget` entry point in package.json.
 */

"use strict";

const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const binName = process.platform === "win32" ? "stackget.exe" : "stackget";
const binPath = path.join(__dirname, "bin", binName);

if (!fs.existsSync(binPath)) {
  console.error(
    "\nstackget binary not found in " + path.join(__dirname, "bin") + "\n" +
    "Try reinstalling:  npm install -g stackget\n"
  );
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: false,
});

// spawnSync returns null status on signal termination — treat as error.
process.exit(result.status ?? 1);

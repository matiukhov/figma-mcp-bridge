#!/usr/bin/env node

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const BINARY_DIR = path.join(__dirname, "binaries");

function getBinaryName() {
  const platform = os.platform(); // 'darwin', 'linux', 'win32'
  const arch = os.arch(); // 'x64', 'arm64'

  // Map Node.js platform/arch names to Go-style names
  const platformMap = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows",
  };

  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const goPlatform = platformMap[platform];
  const goArch = archMap[arch];

  if (!goPlatform) {
    console.error(`Unsupported platform: ${platform}`);
    console.error("Supported platforms: macOS (darwin), Linux, Windows");
    process.exit(1);
  }

  if (!goArch) {
    console.error(`Unsupported architecture: ${arch}`);
    console.error("Supported architectures: x64 (amd64), arm64");
    process.exit(1);
  }

  let name = `figma-map-${goPlatform}-${goArch}`;
  if (platform === "win32") {
    name += ".exe";
  }

  return name;
}

function main() {
  const binaryName = getBinaryName();
  const binaryPath = path.join(BINARY_DIR, binaryName);

  if (!fs.existsSync(binaryPath)) {
    console.error(`Binary not found: ${binaryPath}`);
    console.error(
      `Expected binary "${binaryName}" in the binaries/ directory.`
    );
    console.error(
      "Please download the correct release for your platform from:"
    );
    console.error(
      "https://github.com/gethopp/figma-mcp-bridge/releases"
    );
    process.exit(1);
  }

  // Ensure the binary is executable (no-op on Windows)
  try {
    fs.chmodSync(binaryPath, 0o755);
  } catch {
    // Ignore chmod errors on Windows
  }

  // Forward all arguments and stdio to the binary
  try {
    execFileSync(binaryPath, process.argv.slice(2), {
      stdio: "inherit",
    });
  } catch (err) {
    // execFileSync throws on non-zero exit codes; forward the exit code
    process.exit(err.status ?? 1);
  }
}

main();

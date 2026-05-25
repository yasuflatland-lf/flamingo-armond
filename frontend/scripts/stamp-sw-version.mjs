// Pre-build step: generate `public/sw.js` from `sw-template.js` with a
// deterministic version string. Run before every dev/build so the Service
// Worker's cache name (`flamingo-precache-<version>`) changes only when the
// inputs that define the cached app shell change. Uses Node built-ins only.

import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Resolve paths relative to THIS script, not the current working directory,
// so the script behaves the same whether invoked from `frontend/` or the repo
// root. `scripts/` → `frontend/` is the parent directory.
const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(scriptDir, "..");

const templatePath = path.join(frontendRoot, "sw-template.js");
const outputPath = path.join(frontendRoot, "public", "sw.js");

// The hash inputs, in a fixed order. The template is hashed WITH its
// `__SW_VERSION__` placeholder still in place: that keeps the version a pure
// function of the inputs, so identical inputs always yield the same version.
const hashInputs = [
  templatePath,
  path.join(frontendRoot, "public", "offline.html"),
  path.join(frontendRoot, "public", "icon-192.png"),
  path.join(frontendRoot, "public", "icon-512.png"),
];

// Concatenate the raw bytes of every input and take the first 8 hex chars of
// the SHA-256 digest as the version.
const hash = createHash("sha256");
for (const file of hashInputs) {
  hash.update(readFileSync(file));
}
const version = hash.digest("hex").slice(0, 8);

// Substitute every placeholder occurrence and emit the generated worker.
const template = readFileSync(templatePath, "utf8");
const stamped = template.replaceAll("__SW_VERSION__", version);
writeFileSync(outputPath, stamped);

console.log(`stamped public/sw.js (SW_VERSION=${version})`);

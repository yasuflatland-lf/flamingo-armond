#!/usr/bin/env node
/**
 * Asserts that en.json and ja.json have exactly the same flattened key set.
 * Exits 0 when they match; exits 1 and prints diffs when they diverge.
 * Run: node scripts/i18n-parity.mjs
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const messagesDir = join(__dirname, "../messages");

function flatten(obj, prefix = "") {
  const out = [];
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === "object" && !Array.isArray(v)) {
      out.push(...flatten(v, key));
    } else {
      out.push(key);
    }
  }
  return out.sort();
}

const en = JSON.parse(readFileSync(join(messagesDir, "en.json"), "utf8"));
const ja = JSON.parse(readFileSync(join(messagesDir, "ja.json"), "utf8"));

const enKeys = new Set(flatten(en));
const jaKeys = new Set(flatten(ja));

const onlyEn = [...enKeys].filter((k) => !jaKeys.has(k));
const onlyJa = [...jaKeys].filter((k) => !enKeys.has(k));

let ok = true;

if (onlyEn.length > 0) {
  ok = false;
  console.error("Keys in en.json but missing from ja.json:");
  for (const k of onlyEn) console.error(`  - ${k}`);
}

if (onlyJa.length > 0) {
  ok = false;
  console.error("Keys in ja.json but missing from en.json:");
  for (const k of onlyJa) console.error(`  + ${k}`);
}

if (ok) {
  console.log(`Parity OK — ${enKeys.size} keys in both catalogs.`);
  process.exit(0);
} else {
  process.exit(1);
}

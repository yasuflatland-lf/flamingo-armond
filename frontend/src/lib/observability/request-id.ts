export const REQUEST_ID_HEADER = "X-Request-ID";

// Detect Web Crypto availability once at module load. Older Safari, certain
// JSDOM configurations, and some Node environments expose a `crypto` global
// without `getRandomValues`, so we check the callable, not just the object.
const _cryptoAvailable =
  typeof globalThis.crypto !== "undefined" &&
  typeof globalThis.crypto.getRandomValues === "function";

if (!_cryptoAvailable) {
  console.warn(
    "[request-id] Web Crypto unavailable; falling back to Math.random-derived IDs (collision resistance reduced)",
  );
}

let lastTimestampMs = 0;

export function newRequestId(): string {
  const timestampMs = Math.max(Date.now(), lastTimestampMs);
  lastTimestampMs = timestampMs;

  const timestamp = timestampMs.toString(16).padStart(12, "0").slice(-12);
  const random = randomHex(10);
  const raw = `${timestamp}${random}`;
  const withVersion = `${raw.slice(0, 12)}7${raw.slice(13)}`;
  const variant = ((Number.parseInt(withVersion[16] ?? "0", 16) & 0x3) | 0x8).toString(16);
  const uuid = `${withVersion.slice(0, 16)}${variant}${withVersion.slice(17)}`;

  return `${uuid.slice(0, 8)}-${uuid.slice(8, 12)}-${uuid.slice(12, 16)}-${uuid.slice(16, 20)}-${uuid.slice(20)}`;
}

function randomHex(byteLength: number): string {
  const bytes = new Uint8Array(byteLength);
  if (_cryptoAvailable) {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    // Fallback: fill bytes with Math.random()-derived values. Collision
    // resistance is reduced, but the UUID v7 format contract is preserved.
    for (let i = 0; i < bytes.length; i++) {
      bytes[i] = Math.floor(Math.random() * 256);
    }
  }
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

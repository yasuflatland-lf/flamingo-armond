export const REQUEST_ID_HEADER = "X-Request-ID";

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
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

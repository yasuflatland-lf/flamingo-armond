export const REQUEST_ID_HEADER = "X-Request-ID";

export function newRequestId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);

  const ms = Date.now();
  bytes[0] = (ms / 2 ** 40) & 0xff;
  bytes[1] = (ms / 2 ** 32) & 0xff;
  bytes[2] = (ms / 2 ** 24) & 0xff;
  bytes[3] = (ms / 2 ** 16) & 0xff;
  bytes[4] = (ms / 2 ** 8) & 0xff;
  bytes[5] = ms & 0xff;

  // biome-ignore lint/style/noNonNullAssertion: bytes is Uint8Array(16), index 6 always exists
  bytes[6] = (bytes[6]! & 0x0f) | 0x70;
  // biome-ignore lint/style/noNonNullAssertion: bytes is Uint8Array(16), index 8 always exists
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;

  const h: string[] = Array.from(bytes, (b) => b.toString(16).padStart(2, "0"));
  return (
    `${h[0] ?? ""}${h[1] ?? ""}${h[2] ?? ""}${h[3] ?? ""}-` +
    `${h[4] ?? ""}${h[5] ?? ""}-` +
    `${h[6] ?? ""}${h[7] ?? ""}-` +
    `${h[8] ?? ""}${h[9] ?? ""}-` +
    `${h[10] ?? ""}${h[11] ?? ""}${h[12] ?? ""}${h[13] ?? ""}${h[14] ?? ""}${h[15] ?? ""}`
  );
}

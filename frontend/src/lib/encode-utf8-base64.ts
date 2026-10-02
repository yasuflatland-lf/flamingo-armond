// String.fromCharCode(...spread) over the whole byte array overflows the call stack on large payloads.
const CHUNK_SIZE = 0x8000;

/**
 * Encodes text as standard (padded) base64 of its UTF-8 bytes. Never throws on
 * malformed UTF-16: TextEncoder replaces unpaired surrogates with U+FFFD. Only an
 * input past the engine's maximum string length can throw (RangeError). Matches the
 * backend's base64.StdEncoding decode of card-import payloads.
 */
export function encodeUtf8Base64(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = "";
  for (let i = 0; i < bytes.length; i += CHUNK_SIZE) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK_SIZE));
  }
  return btoa(binary);
}

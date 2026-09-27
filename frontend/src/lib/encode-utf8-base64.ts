/**
 * Encodes text as standard (padded) base64 of its UTF-8 bytes. Total over every
 * DOMString: TextEncoder replaces unpaired surrogates with U+FFFD, so this never
 * throws. Matches the backend's base64.StdEncoding decode of card-import payloads.
 */
export function encodeUtf8Base64(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = "";
  // String.fromCharCode(...spread) on a very large array overflows the call stack.
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  }
  return btoa(binary);
}

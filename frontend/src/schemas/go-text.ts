// String.prototype.trim strips U+FEFF and keeps U+0085, unlike Go's strings.TrimSpace.
const GO_SPACE_EDGES =
  /^[\t\n\v\f\r \u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000]+|[\t\n\v\f\r \u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000]+$/g;

export function trimLikeGo(s: string): string {
  return s.replace(GO_SPACE_EDGES, "");
}

// Whole-string JS lowercasing expands U+0130 and applies final-sigma context;
// Go's strings.ToLower uses simple per-code-point mappings instead.
export function toLowerLikeGo(s: string): string {
  return Array.from(s, (c) => (c === "\u0130" ? "i" : c.toLowerCase())).join("");
}

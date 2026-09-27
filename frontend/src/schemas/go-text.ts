// One /^[...]+|[...]+$/g regex backtracks quadratically on a long interior
// whitespace run; every Go space is in the BMP, so a per-code-unit scan is exact.
const GO_SPACE = /[\t\n\v\f\r \u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000]/;

// String.prototype.trim strips U+FEFF and keeps U+0085, unlike Go's strings.TrimSpace.
export function trimLikeGo(s: string): string {
  let start = 0;
  let end = s.length;
  while (start < end && GO_SPACE.test(s.charAt(start))) start++;
  while (end > start && GO_SPACE.test(s.charAt(end - 1))) end--;
  return s.slice(start, end);
}

// Whole-string JS lowercasing expands U+0130 and applies final-sigma context;
// Go's strings.ToLower uses simple per-code-point mappings instead.
export function toLowerLikeGo(s: string): string {
  return Array.from(s, (c) => (c === "\u0130" ? "i" : c.toLowerCase())).join("");
}

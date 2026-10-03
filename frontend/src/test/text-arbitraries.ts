import fc from "fast-check";

/** The 25 code points Go `strings.TrimSpace` strips (`unicode.IsSpace`), one per element. */
export const GO_SPACES = Array.from(
  "\t\n\v\f\r \u0085\u00A0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006" +
    "\u2007\u2008\u2009\u200A\u2028\u2029\u202F\u205F\u3000",
);

/**
 * Each entry is exactly one UAX #29 grapheme and never merges with a neighbour
 * drawn from this list or from GO_SPACES, so a joined core has a known count.
 * No reserved display name can be assembled from them.
 */
const GRAPHEMES = [
  "a",
  "Z",
  "7",
  "-",
  "\u00E9",
  "e\u0301",
  "\u{1F468}\u200D\u{1F469}\u200D\u{1F467}\u200D\u{1F466}",
  "\u{1F1EF}\u{1F1F5}",
] as const;

type PaddedText = { raw: string; trimmed: string; graphemes: number };

/** Go-space padding + a core of known grapheme count, biased to 0 and to max +- 2. */
export function paddedText(max: number): fc.Arbitrary<PaddedText> {
  const pad = fc.string({ unit: fc.constantFrom(...GO_SPACES), maxLength: 3 });
  const count = fc.oneof(
    fc.integer({ min: 0, max: 2 }),
    fc.integer({ min: Math.max(0, max - 2), max: max + 2 }),
    fc.integer({ min: 0, max: max + 2 }),
  );
  const core = count.chain((n) =>
    fc.array(fc.constantFrom(...GRAPHEMES), { minLength: n, maxLength: n }),
  );
  return fc.tuple(pad, core, pad).map(([lead, parts, trail]) => {
    const trimmed = parts.join("");
    return { raw: lead + trimmed + trail, trimmed, graphemes: parts.length };
  });
}

type SafeParse =
  | { success: true; data: unknown }
  | { success: false; error: { issues: { message: string }[] } };

/**
 * Law for a trimmed, bounded text field: accepted iff the Go-trimmed text has
 * 1..max graphemes (0..max when `required` is omitted), the output is the
 * trimmed text, and each rejection carries its exact message.
 */
export function assertBoundedTextLaw(
  parse: (raw: string) => SafeParse,
  pick: (data: never) => unknown,
  { max, required, tooLong }: { max: number; required?: string; tooLong: string },
): void {
  fc.assert(
    fc.property(paddedText(max), ({ raw, trimmed, graphemes }) => {
      const r = parse(raw);
      const messages = r.success ? [] : r.error.issues.map((i) => i.message);
      if (graphemes === 0 && required !== undefined) return messages.includes(required);
      if (graphemes > max) return messages.includes(tooLong);
      return r.success && pick(r.data as never) === trimmed;
    }),
  );
}

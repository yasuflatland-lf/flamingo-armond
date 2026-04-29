// UAX #29 grapheme cluster counter shared by schema length validators
// to keep FE and BE rules in sync (ZWJ emoji count as 1 grapheme).
const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

export function graphemeCount(s: string): number {
  return Array.from(segmenter.segment(s)).length;
}

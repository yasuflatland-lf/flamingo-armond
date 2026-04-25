import { z } from "zod";

// Mirrors UpdateProfileInput in schema/schema.graphql; bio: undefined = unchanged, "" = explicit clear.
// UAX #29 grapheme cluster counting keeps FE and BE length rules in sync.
const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

function graphemeCount(s: string): number {
  let n = 0;
  for (const _segment of segmenter.segment(s)) {
    n += 1;
  }
  return n;
}

const displayName = z
  .string()
  .trim()
  .refine((s) => graphemeCount(s) >= 1, { message: "Display name is required" })
  .refine((s) => graphemeCount(s) <= 50, {
    message: "Display name must be 50 characters or fewer",
  });

const bio = z
  .string()
  .optional()
  .refine((s) => s === undefined || graphemeCount(s) <= 500, {
    message: "Bio must be 500 characters or fewer",
  });

export const updateProfileSchema = z.object({
  displayName,
  bio,
});

export type UpdateProfileFormValues = z.infer<typeof updateProfileSchema>;

import { z } from "zod";

// Mirrors NewCardgroupInput / UpdateCardgroupInput in schema/schema.graphql.
// UAX #29 grapheme cluster counting keeps FE and BE length rules in sync.
const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

function graphemeCount(s: string): number {
  return Array.from(segmenter.segment(s)).length;
}

const cardgroupNameSchema = z
  .string()
  .trim()
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= 100, {
    message: "name must be at most 100 characters",
  });

export const newCardgroupSchema = z.object({
  name: cardgroupNameSchema,
});

export const updateCardgroupSchema = z.object({
  name: cardgroupNameSchema,
});

export type NewCardgroupValues = z.infer<typeof newCardgroupSchema>;
export type UpdateCardgroupValues = z.infer<typeof updateCardgroupSchema>;

import { z } from "zod";

// Mirrors NewCardInput / UpdateCardInput in schema/schema.graphql.
// UAX #29 grapheme cluster counting keeps FE and BE length rules in sync.
const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

function graphemeCount(s: string): number {
  return Array.from(segmenter.segment(s)).length;
}

const cardSideSchema = (fieldName: string) =>
  z
    .string()
    .trim()
    .refine((s) => graphemeCount(s) >= 1, { message: `${fieldName} is required` })
    .refine((s) => graphemeCount(s) <= 500, {
      message: `${fieldName} must be at most 500 characters`,
    });

export const newCardSchema = z.object({
  cardgroupId: z.string().min(1, { message: "cardgroupId is required" }),
  front: cardSideSchema("front"),
  back: cardSideSchema("back"),
});

// Both front and back are required in the form even for updates;
// the form always re-submits both fields for simplicity.
export const updateCardSchema = z.object({
  front: cardSideSchema("front"),
  back: cardSideSchema("back"),
});

export type NewCardValues = z.infer<typeof newCardSchema>;
export type UpdateCardValues = z.infer<typeof updateCardSchema>;

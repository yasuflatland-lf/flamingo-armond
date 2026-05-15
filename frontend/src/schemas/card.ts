import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Mirrors NewCardInput / UpdateCardInput in schema/schema.graphql.
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

import { z } from "zod";
import { trimLikeGo } from "./go-text";
import { graphemeCount } from "./grapheme";

// Mirrors NewCardInput / UpdateCardInput in schema/schema.graphql.
const cardSideSchema = (fieldName: string) =>
  z
    .string()
    .overwrite(trimLikeGo)
    .refine((s) => graphemeCount(s) >= 1, { message: `${fieldName} is required` })
    .refine((s) => graphemeCount(s) <= 500, {
      message: `${fieldName} must be at most 500 characters`,
    });

// Both front and back are required in the form even for updates;
// the form always re-submits both fields for simplicity.
export const cardSchema = z.object({
  front: cardSideSchema("front"),
  back: cardSideSchema("back"),
});

import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Mirrors NewCardgroupInput / UpdateCardgroupInput in schema/schema.graphql.
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

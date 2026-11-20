import { z } from "zod";
import { trimLikeGo } from "./go-text";
import { graphemeCount } from "./grapheme";

// Mirrors NewCardgroupInput / UpdateCardgroupInput in schema/schema.graphql.
const cardgroupNameSchema = z
  .string()
  .overwrite(trimLikeGo)
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= 100, {
    message: "name must be at most 100 characters",
  });

export const cardgroupSchema = z.object({ name: cardgroupNameSchema });

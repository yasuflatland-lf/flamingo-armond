import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Mirrors UpdateProfileInput in schema/schema.graphql; bio: undefined = unchanged, "" = explicit clear.
// UAX #29 grapheme cluster counting keeps FE and BE length rules in sync.
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

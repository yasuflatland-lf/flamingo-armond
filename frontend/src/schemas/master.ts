import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Mirrors the backend master name rule: trimmed, 1-100 grapheme clusters.
const masterNameSchema = z
  .string()
  .transform((s) => s.trim())
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= 100, {
    message: "name must be at most 100 characters",
  });

// Single schema validates the only client-enforced field. The remaining
// optional attributes (description, ...) are free-form strings the backend
// validates; the form does not duplicate those rules.
export const masterSchema = z.object({
  name: masterNameSchema,
});

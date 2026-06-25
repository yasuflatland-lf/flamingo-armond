import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Mirrors the backend master name rule: trimmed, 1-100 grapheme clusters.
// Exported (non-optional, input: string) so the form consumes it as a per-field
// TanStack Form validator, which requires a StandardSchema over `string`.
export const masterNameSchema = z
  .string()
  .transform((s) => s.trim())
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= 100, {
    message: "name must be at most 100 characters",
  });

// Mirrors the backend Description value object (domain.DescriptionMax): trimmed,
// optional, at most 500 grapheme clusters. Empty collapses to "no description".
export const masterDescriptionSchema = z
  .string()
  .transform((s) => s.trim())
  .refine((s) => graphemeCount(s) <= 500, {
    message: "description must be at most 500 characters",
  });

// The sort-order form input is a string; allow empty (→ null on submit) or a
// finite whole number. NaN, Infinity, and decimals are rejected before submit.
// Frontend-only guard: the GraphQL Int type already bounds the value server-side,
// so there is no backend value object to mirror.
export const masterSortOrderSchema = z.string().refine(
  (s) => {
    const trimmed = s.trim();
    if (trimmed === "") return true;
    return Number.isInteger(Number(trimmed));
  },
  { message: "sort order must be a whole number" },
);

// name is always client-validated; description and sortOrder are optional so the
// whole-object parse stays valid when a caller supplies only the fields it owns.
// The form consumes each per-field schema via the standalone exports above (a
// per-field validator needs a StandardSchema over `string`, which `.optional()`
// here would widen to `string | undefined`).
export const masterSchema = z.object({
  name: masterNameSchema,
  description: masterDescriptionSchema.optional(),
  sortOrder: masterSortOrderSchema.optional(),
});

import { z } from "zod";
import { trimLikeGo } from "./go-text";
import { graphemeCount } from "./grapheme";

// Mirrors the backend master name rule: trimmed, 1-100 grapheme clusters.
// Exported (non-optional, input: string) so the form consumes it as a per-field
// TanStack Form validator, which requires a StandardSchema over `string`.
export const masterNameSchema = z
  .string()
  .transform(trimLikeGo)
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= 100, {
    message: "name must be at most 100 characters",
  });

// Mirrors the backend Description value object (domain.DescriptionMax): trimmed,
// optional, at most 500 grapheme clusters. Empty collapses to "no description".
export const masterDescriptionSchema = z
  .string()
  .transform(trimLikeGo)
  .refine((s) => graphemeCount(s) <= 500, {
    message: "description must be at most 500 characters",
  });

// Allow empty (null on submit); otherwise require plain base-10 digits because
// Number() alone accepts "0x10", "1e3" and "+5", which would be saved as 16, 1000, 5.
// The int32 bound matches the Postgres integer column. GraphQL Int maps to Go int
// (64-bit) with no domain bound, so an out-of-range value would otherwise surface
// as a generic server error.
const SORT_ORDER_MIN = -2147483648;
const SORT_ORDER_MAX = 2147483647;

export const masterSortOrderSchema = z.string().refine(
  (s) => {
    const trimmed = trimLikeGo(s);
    if (trimmed === "") return true;
    if (!/^-?\d+$/.test(trimmed)) return false;
    const n = Number(trimmed);
    return n >= SORT_ORDER_MIN && n <= SORT_ORDER_MAX;
  },
  { message: `sort order must be a whole number between ${SORT_ORDER_MIN} and ${SORT_ORDER_MAX}` },
);

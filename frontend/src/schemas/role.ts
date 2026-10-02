import { z } from "zod";
import { toLowerLikeGo, trimLikeGo } from "./go-text";
import { graphemeCount } from "./grapheme";

// Mirrors the backend `ParseRoleName` rules in backend/internal/domain/role_name.go:
// trim + lowercase, 1-50 characters (RoleNameMax counts bytes, which equals
// graphemes because the [a-z0-9_-] pattern is ASCII-only), and that character
// set. The transform runs before the refinements so the checks see the
// server's canonical form.
const roleNameSchema = z
  .string()
  .transform((s) => toLowerLikeGo(trimLikeGo(s)))
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= 50, {
    message: "name must be at most 50 characters",
  })
  .refine((s) => /^[a-z0-9_-]+$/.test(s), {
    message: "name must contain only lowercase letters, digits, '_' or '-'",
  });

// Single object schema serves both create and edit flows: the rules are
// identical (the server applies the same `ParseRoleName` for createRole
// and updateRole), so a parallel "update" schema would be a duplicate.
export const roleSchema = z.object({
  name: roleNameSchema,
});

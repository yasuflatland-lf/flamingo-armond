import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Mirrors UpdateProfileInput in schema/schema.graphql; bio: undefined = unchanged, "" = explicit clear.
// UAX #29 grapheme cluster counting keeps FE and BE length rules in sync.
export const DISPLAY_NAME_MAX = 50;
export const BIO_MAX = 500;

// Advisory mirror of backend/internal/domain/display_name.go `reservedDisplayNames`.
// The backend (`domain.ParseDisplayName`) remains the sole authority; this set only
// gives instant inline feedback so a reserved name need not cost a server round-trip.
// Exact lowercased equality only — no substring matching. Includes the lowercased
// role-name constants (`admin` = AdminRoleName, `general` = GeneralRoleName).
const reservedDisplayNames = new Set([
  "admin",
  "administrator",
  "root",
  "system",
  "support",
  "moderator",
  "mod",
  "staff",
  "official",
  "flamingo",
  "flamingo-armond",
  "general",
]);

const displayName = z
  .string()
  .trim()
  .refine((s) => graphemeCount(s) >= 1, { message: "Display name is required" })
  .refine((s) => graphemeCount(s) <= DISPLAY_NAME_MAX, {
    message: `Display name must be ${DISPLAY_NAME_MAX} characters or fewer`,
  })
  .refine((s) => !reservedDisplayNames.has(s.toLowerCase()), {
    message: "Display name is reserved",
  });

const bio = z
  .string()
  .optional()
  .refine((s) => s === undefined || graphemeCount(s) <= BIO_MAX, {
    message: `Bio must be ${BIO_MAX} characters or fewer`,
  });

export const updateProfileSchema = z.object({
  displayName,
  bio,
});

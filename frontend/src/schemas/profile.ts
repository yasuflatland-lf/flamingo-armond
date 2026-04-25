import { z } from "zod";

// Mirrors UpdateProfileInput in schema/schema.graphql.
// displayName: required, 1-50 characters after trim.
// bio: optional. undefined = leave unchanged, "" = explicit clear.
export const updateProfileSchema = z.object({
  displayName: z
    .string()
    .trim()
    .min(1, "Display name is required")
    .max(50, "Display name must be 50 characters or fewer"),
  bio: z.string().max(500, "Bio must be 500 characters or fewer").optional(),
});

export type UpdateProfileFormValues = z.infer<typeof updateProfileSchema>;

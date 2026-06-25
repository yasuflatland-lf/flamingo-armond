import { z } from "zod";
import { graphemeCount } from "./grapheme";

// Caps mirror the backend domain bounds (Master*Max / CoverImageURLMax).
const NAME_MAX = 100;
const DESCRIPTION_MAX = 1000;
const LANGUAGE_MAX = 50;
const LEVEL_MAX = 50;
const CATEGORY_MAX = 100;
const SOURCE_MAX = 500;
const COVER_IMAGE_URL_MAX = 2048;

// Mirrors the backend master name rule: trimmed, 1-100 grapheme clusters.
const masterNameSchema = z
  .string()
  .transform((s) => s.trim())
  .refine((s) => graphemeCount(s) >= 1, { message: "name is required" })
  .refine((s) => graphemeCount(s) <= NAME_MAX, {
    message: `name must be at most ${NAME_MAX} characters`,
  });

// Optional bounded text: undefined/empty -> null; otherwise grapheme count <= max.
const optionalBounded = (max: number, field: string) =>
  z
    .string()
    .optional()
    .transform((s) => (s ?? "").trim())
    .transform((s) => (s === "" ? null : s))
    .refine((s) => s === null || graphemeCount(s) <= max, {
      message: `${field} must be at most ${max} characters`,
    });

function isHttpUrl(s: string): boolean {
  let u: URL;
  try {
    u = new URL(s);
  } catch {
    return false;
  }
  return u.protocol === "http:" || u.protocol === "https:";
}

// Optional cover image URL: undefined/empty -> null; otherwise length-bounded
// http(s) URL. Rejecting javascript:/data: closes latent XSS at the form layer.
const coverImageUrlSchema = z
  .string()
  .optional()
  .transform((s) => (s ?? "").trim())
  .transform((s) => (s === "" ? null : s))
  .refine((s) => s === null || s.length <= COVER_IMAGE_URL_MAX, {
    message: `coverImageUrl must be at most ${COVER_IMAGE_URL_MAX} characters`,
  })
  .refine((s) => s === null || isHttpUrl(s), {
    message: "coverImageUrl must be a valid http or https URL",
  });

// Optional sortOrder held as a string by the form: undefined/empty -> null;
// otherwise reject non-integer / NaN / Infinity before submit.
const sortOrderSchema = z
  .string()
  .optional()
  .transform((s) => (s ?? "").trim())
  .transform((s) => (s === "" ? null : s))
  .refine((s) => s === null || /^-?\d+$/.test(s), {
    message: "sortOrder must be a whole number",
  })
  .transform((s) => (s === null ? null : Number(s)));

export const masterSchema = z.object({
  name: masterNameSchema,
  description: optionalBounded(DESCRIPTION_MAX, "description"),
  language: optionalBounded(LANGUAGE_MAX, "language"),
  level: optionalBounded(LEVEL_MAX, "level"),
  category: optionalBounded(CATEGORY_MAX, "category"),
  coverImageUrl: coverImageUrlSchema,
  source: optionalBounded(SOURCE_MAX, "source"),
  sortOrder: sortOrderSchema,
});

/**
 * Fixed UTC+9 offset, in milliseconds, used to locate the learner's start-of-day
 * boundary. JST observes no daylight saving, so a fixed offset is exact and
 * needs no tzdata lookup. This mirrors `learnDayZone` in
 * `backend/internal/domain/learn_day.go`; the product assumes a Japan-resident
 * learner on both sides.
 */
const LEARN_DAY_OFFSET_MS = 9 * 60 * 60 * 1000;

/**
 * Returns the canonical JST learn-day key (`YYYY-MM-DD`) for `now`.
 *
 * The key rolls over at JST midnight — 15:00 UTC — which is the same boundary
 * the server's `StartOfLearnDay` computes for the learn queue's review windows.
 * Client state scoped to "today" (a session-scoped swiped-id set, an exhaustion
 * verdict) keys off this value so it is dropped when the server starts serving
 * a new learn day.
 *
 * The offset is added before formatting so the UTC calendar date of the shifted
 * instant is the JST calendar date of the original one.
 */
export function learnDayKey(now: Date = new Date()): string {
  return new Date(now.getTime() + LEARN_DAY_OFFSET_MS).toISOString().slice(0, 10);
}

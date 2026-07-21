// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/**
 * The ids of every card swiped so far in the current learn session, tagged with
 * the JST learn day (`YYYY-MM-DD`, see `learnDayKey`) they were recorded under.
 *
 * The day tag is part of the value rather than an implicit assumption of the
 * caller so `mergePrefetchedCards` is total: it can decide on its own whether a
 * given swiped-id set still applies to the learn day being merged for.
 */
export type SwipedSession = {
  readonly dayKey: string;
  readonly ids: ReadonlySet<string>;
};

/**
 * Outcome of merging one resolved prefetch batch into the swipe queue.
 *
 * - `queue` — the next queue. Always a freshly allocated array; the inputs are
 *   never mutated.
 * - `exhausted` — `true` when the batch added nothing, meaning the due pool has
 *   run dry for now and further tail swipes should not re-fire the query. It is
 *   a point-in-time verdict, not a permanent one; deciding when to clear it is
 *   the caller's concern.
 */
export type PrefetchMerge<T> = {
  readonly queue: T[];
  readonly exhausted: boolean;
};

/**
 * Shared empty set used when the recorded swipes belong to a past learn day.
 * Typed `ReadonlySet` and never written to, so sharing one instance is safe.
 */
const NO_SWIPED_IDS: ReadonlySet<string> = new Set<string>();

// ---------------------------------------------------------------------------
// mergePrefetchedCards
// ---------------------------------------------------------------------------

/**
 * Pure, side-effect-free policy for merging a resolved background-prefetch
 * batch into the learn session's swipe queue.
 *
 * Three verdicts
 * ──────────────
 * 1. `incoming` is empty — the server has nothing more to serve. The queue is
 *    returned unchanged and the pool is marked exhausted.
 * 2. Every incoming row is filtered out — the batch carried only cards already
 *    queued or already swiped this learn day, so it adds nothing. Same verdict
 *    as (1): the merge made no progress, so re-querying on the next tail swipe
 *    would just repeat the round trip.
 * 3. Some rows survive — they are appended to the tail in the order the server
 *    returned them, and the pool is not exhausted.
 *
 * Why a swiped card can come back at all
 * ──────────────────────────────────────
 * The consumer removes a card from the queue optimistically, which can re-fire
 * the prefetch WHILE the swipe mutation has not yet committed its FSRS write. A
 * `network-only` read that beats that write still sees the card as due and
 * returns it in the batch. Re-appending it would let the learner rate the same
 * card twice and record a duplicate same-day FSRS review, so `swiped.ids` is
 * filtered out alongside the ids already in `current`.
 *
 * Why swiped ids are day-keyed rather than pruned one by one
 * ─────────────────────────────────────────────────────────
 * Both server-side REVIEW windows require `last_review < StartOfLearnDay(now)`
 * (see the contract on `StartOfLearnDay` in
 * `backend/internal/domain/learn_day.go`), and the new-card window carries no
 * `last_review` predicate at all — it matches only cards with no FSRS row,
 * which the swipe itself creates. So once a swipe commits, no window can return
 * that card again WITHIN THE SAME JST LEARN DAY: any batch carrying one of
 * those ids is a stale read that predates the commit, and the id can stay in
 * the set for the whole day.
 *
 * That invariant expires at the learn-day rollover. `StartOfLearnDay` advances
 * at JST midnight, so a session held open past it sees the server legitimately
 * re-serve cards swiped on the previous day. When `swiped.dayKey` is not
 * `learnDay`, the recorded ids belong to a finished day and are ignored
 * wholesale — otherwise every re-served card would be filtered out and the
 * refilled pool would be misread as exhausted.
 *
 * The day is an explicit parameter rather than an assumption because the policy
 * has to be total over its inputs: a caller that drops its swiped-id set at the
 * rollover and a caller that lets the tag go stale must both get the same
 * answer. `LearnClient` is the former — it re-keys the set before recording any
 * swipe — so its own merges always pass a matching pair.
 *
 * @param current   Current ordered queue. Never mutated.
 * @param incoming  The batch the server just returned, in server order.
 * @param swiped    Ids swiped this session plus the learn day they belong to.
 * @param learnDay  The learn-day key the merge is being performed for.
 * @returns         The next queue plus the exhaustion verdict.
 */
export function mergePrefetchedCards<T extends { id: string }>(
  current: readonly T[],
  incoming: readonly T[],
  swiped: SwipedSession,
  learnDay: string,
): PrefetchMerge<T> {
  if (incoming.length === 0) {
    return { queue: [...current], exhausted: true };
  }

  const seen = new Set(current.map((card) => card.id));
  const swipedIds = swiped.dayKey === learnDay ? swiped.ids : NO_SWIPED_IDS;
  const additions = incoming.filter((card) => !seen.has(card.id) && !swipedIds.has(card.id));

  if (additions.length === 0) {
    return { queue: [...current], exhausted: true };
  }

  return { queue: [...current, ...additions], exhausted: false };
}

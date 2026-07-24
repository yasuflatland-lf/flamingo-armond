package domain

import "time"

// learnDayZone is the fixed UTC+9 offset used to compute the learner's
// start-of-day boundary. JST observes no daylight saving, so a fixed offset
// is exact and avoids a tzdata dependency. The product currently assumes a
// Japan-resident learner; promote to a per-user preference if that breaks.
var learnDayZone = time.FixedZone("JST", 9*60*60)

// StartOfLearnDay returns the JST midnight at or before now. The learn queue's
// review window includes only cards whose last_review is strictly before this
// boundary, so a card swiped today never re-enters today's queue regardless of
// its FSRS re-due interval. JST observes no DST, so a fixed offset is exact.
func StartOfLearnDay(now time.Time) time.Time {
	local := now.In(learnDayZone)
	y, m, d := local.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, learnDayZone)
}

// EndOfLearnDay returns the JST midnight strictly after now — the exclusive
// upper bound of the current learn day. JST has no DST, so Add(24h) is exact.
func EndOfLearnDay(now time.Time) time.Time {
	return StartOfLearnDay(now).Add(24 * time.Hour)
}

// rescueMinElapsed is the minimum wall-clock time that must pass after a review
// before the same card may be served early through the rescue window. The FSRS
// memory model derives elapsed days as floor(hours/24), so a repeat inside the
// same 24 hours counts as zero elapsed days: retrievability is 1 and the
// stability growth factor exp((1-r)*W10)-1 is exactly 0. Such a review earns no
// scheduling credit at all, so serving the card early only burns a rescue slot
// while leaving the card below the learned threshold.
const rescueMinElapsed = 24 * time.Hour

// RescueReviewedBefore returns the latest last_review instant a card may carry
// and still be served early through the rescue window: exactly rescueMinElapsed
// before now. The repository compares with `<=`, so a card last reviewed exactly
// 24 hours ago is eligible while one reviewed 23 hours ago is not.
func RescueReviewedBefore(now time.Time) time.Time {
	return now.Add(-rescueMinElapsed)
}

// LearnDayKey returns the canonical JST learn-day key (YYYY-MM-DD) for t.
func LearnDayKey(t time.Time) string { return StartOfLearnDay(t).Format(time.DateOnly) }

// LearnWindow bundles the four instants that bound one learn-queue fetch,
// all derived from a single now. Field -> SQL comparator mapping
// (findDueCardsOn in repository/card_due.go):
//
//	Now                  — filler:  ucs.due <= Now
//	ReviewedBefore       — rescue+filler: ucs.last_review < ReviewedBefore (StartOfLearnDay)
//	RescueDueBefore      — rescue:  ucs.due < RescueDueBefore (EndOfLearnDay, exclusive)
//	RescueReviewedBefore — rescue:  ucs.last_review <= RescueReviewedBefore (24h floor)
//
// Production code must construct via NewLearnWindow; field literals are for
// tests that need non-canonical windows.
type LearnWindow struct {
	Now                  time.Time
	ReviewedBefore       time.Time
	RescueDueBefore      time.Time
	RescueReviewedBefore time.Time
}

// NewLearnWindow derives the canonical learn-queue window from a single now.
func NewLearnWindow(now time.Time) LearnWindow {
	return LearnWindow{
		Now:                  now,
		ReviewedBefore:       StartOfLearnDay(now),
		RescueDueBefore:      EndOfLearnDay(now),
		RescueReviewedBefore: RescueReviewedBefore(now),
	}
}

// ReviewedWithinLearnDay reports whether lastReview falls inside the JST learn
// day containing now: lastReview is at or after StartOfLearnDay(now). It is the
// exact complement of the serving-side SQL predicate
// `ucs.last_review < StartOfLearnDay(now)` in repository/card_due.go
// (findDueCardsOn's rescue and filler windows): a review exactly at the
// boundary counts as reviewed today on both sides.
func ReviewedWithinLearnDay(lastReview, now time.Time) bool {
	return !lastReview.Before(StartOfLearnDay(now))
}

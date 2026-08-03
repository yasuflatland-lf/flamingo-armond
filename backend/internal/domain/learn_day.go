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

// EarnsSchedulingCredit reports whether a review at now advances the FSRS
// schedule for a card last reviewed at lastReview. go-fsrs counts elapsed days as
// a UTC calendar-date difference, so credit is earned exactly when the two
// instants fall on different UTC dates: on the same date retrievability is 1 and
// the recall stability growth factor exp((1-r)*W10)-1 is bit-exactly 0.
//
// Wall-clock distance is not the rule and must not be substituted for it. A
// repeat one hour apart across UTC midnight earns the same stability growth as
// one 24 hours apart, while a repeat 23 hours apart inside a single UTC date
// earns none. A zero LastReview and a backward clock step both earn nothing,
// mirroring the library's own New-card and hours<0 clamps.
func EarnsSchedulingCredit(lastReview, now time.Time) bool {
	if lastReview.IsZero() || now.Before(lastReview) {
		return false
	}
	return !utcCalendarDay(lastReview).Equal(utcCalendarDay(now))
}

// utcCalendarDay truncates t to UTC midnight, the operation go-fsrs applies to
// both operands before differencing them. Constructing the date explicitly rather
// than calling Truncate(24*time.Hour) keeps the mirror obvious: Truncate happens
// to agree only because Go's zero time is itself a UTC midnight.
func utcCalendarDay(t time.Time) time.Time {
	u := t.UTC()
	y, m, d := u.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// RescueReviewedBefore returns the exclusive upper bound on last_review for the
// rescue and filler windows: UTC midnight of now's UTC calendar date. go-fsrs
// counts elapsed days by UTC calendar date, so a card last reviewed strictly
// before this instant earns stability growth at now — the exact condition
// EarnsSchedulingCredit tests on the recording side.
//
// Not a 24-hour rolling floor: that approximation never admitted a zero-credit
// repeat, but it also withheld every card a JST learner reviewed after 09:00 the
// previous day, which has already crossed a UTC date boundary.
func RescueReviewedBefore(now time.Time) time.Time {
	return utcCalendarDay(now)
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
//	RescueReviewedBefore — rescue+filler: ucs.last_review < RescueReviewedBefore (UTC-date credit bound)
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

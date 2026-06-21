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

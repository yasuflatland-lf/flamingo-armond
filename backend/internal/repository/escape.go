package repository

import "strings"

// likeEscaper escapes the three LIKE/ILIKE meta-characters (\, _, %) so a
// user-supplied substring can be embedded inside an outer "%...%" pattern
// safely. Backslash is listed first so it wins the first-match comparison at
// any position that starts with \, preventing double-escaping of the sequences
// this pass emits.
//
// Reference: .claude/rules/go-library-gotchas.md "GORM LIKE / ILIKE
// requires escaping %, _, \ in user input".
var likeEscaper = strings.NewReplacer(
	`\`, `\\`,
	`_`, `\_`,
	`%`, `\%`,
)

// escapeLikePattern escapes the three Postgres LIKE/ILIKE meta-characters
// (\, _, %) in a user-supplied substring so it matches literally when
// embedded inside an outer "%...%" pattern.
func escapeLikePattern(s string) string {
	return likeEscaper.Replace(s)
}

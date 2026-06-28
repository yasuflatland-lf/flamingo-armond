package domain

// GeneralUserCardgroupLimit caps the number of cardgroups a non-admin owner may
// hold. The threshold lives in the domain because it is an availability
// invariant, mirroring IsLastAdmin, not application plumbing. Whether a given
// owner is exempt (admins are) is an authorization concern enforced in the
// usecase layer, not a domain invariant.
const GeneralUserCardgroupLimit = 5

// GeneralUserCardgroupQuotaReached reports whether an owner holding count
// cardgroups has reached the general-user quota. Callers apply the admin
// exemption before invoking this predicate.
func GeneralUserCardgroupQuotaReached(count int64) bool {
	return count >= GeneralUserCardgroupLimit
}

package domain

// RoleSet is a collection of roles assigned to a user. It exists to express
// User/Role invariants (e.g. "this set retains the admin role") as domain
// behaviour rather than usecase-layer plumbing.
type RoleSet []Role

// ContainsAdmin reports whether the set includes the built-in admin role.
// It underpins the self-demotion guard: a self-editing admin's final role set
// must satisfy ContainsAdmin to keep its admin membership.
func (rs RoleSet) ContainsAdmin() bool {
	for _, r := range rs {
		if r.Name == AdminRoleName {
			return true
		}
	}
	return false
}

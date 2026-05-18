package domain

// Role is an application role. User-role membership stays outside the domain
// model and is queried through the repository layer.
type Role struct {
	ID   string
	Name string
}

// AdminRoleName and GeneralRoleName are the two built-in system role names.
const (
	AdminRoleName   = "admin"
	GeneralRoleName = "general"
)

// systemRoleNames is the set of role names that are owned by the system.
var systemRoleNames = map[string]struct{}{
	AdminRoleName:   {},
	GeneralRoleName: {},
}

// IsSystem reports whether r is one of the built-in system roles.
func (r Role) IsSystem() bool {
	_, ok := systemRoleNames[r.Name]
	return ok
}

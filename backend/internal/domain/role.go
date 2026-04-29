package domain

// Role is an application role. User-role membership stays outside the domain
// model and is queried through the repository layer.
type Role struct {
	ID   string
	Name string
}

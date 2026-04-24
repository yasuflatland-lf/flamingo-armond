package resolver

// Resolver is the root dependency-injection container for gqlgen resolvers.
// Kept in a hand-written file so regeneration of *.resolvers.go never touches
// the DI wiring.
type Resolver struct{}

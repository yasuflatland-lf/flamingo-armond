package resolver

// Resolver is the root dependency-injection container for gqlgen resolvers.
//
// Fields are added as subsequent PRs introduce dependencies:
//   - PR8 will add db / repositories.
//   - PR10 will add dataloaders.
//   - PR11 will add tracer / observer.
//
// Keeping it in resolver.go (hand-written) separates intentional DI wiring
// from the auto-regenerated *.resolvers.go files.
type Resolver struct{}

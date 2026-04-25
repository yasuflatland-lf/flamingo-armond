// Package auth provides JWT verification middleware and authenticated-user context access.
package auth

import "context"

type AuthUser struct {
	Sub   string
	Email string
	Role  string
}

type contextKey struct{}

func withUser(ctx context.Context, u *AuthUser) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

// UserFrom returns the authenticated user stored by AuthMiddleware, or nil for anonymous requests.
func UserFrom(ctx context.Context) *AuthUser {
	u, _ := ctx.Value(contextKey{}).(*AuthUser)
	return u
}

// ContextWithUser returns a copy of ctx carrying u. Intended for tests that
// need to simulate an authenticated request without going through the full JWT
// middleware stack.
func ContextWithUser(ctx context.Context, u *AuthUser) context.Context {
	return withUser(ctx, u)
}

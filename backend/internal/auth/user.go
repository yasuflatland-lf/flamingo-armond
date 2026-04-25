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

// ContextWithUser exposes withUser for tests that simulate an authenticated
// request without exercising the JWT middleware.
func ContextWithUser(ctx context.Context, u *AuthUser) context.Context {
	return withUser(ctx, u)
}

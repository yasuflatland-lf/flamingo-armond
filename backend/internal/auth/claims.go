package auth

import "github.com/golang-jwt/jwt/v5"

// supabaseClaims captures the subset of Supabase-issued JWT claims the middleware consumes.
type supabaseClaims struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

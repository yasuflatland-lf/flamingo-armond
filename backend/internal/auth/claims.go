package auth

import "github.com/golang-jwt/jwt/v5"

type supabaseClaims struct {
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Role          string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

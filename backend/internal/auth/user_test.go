package auth

import (
	"context"
	"testing"
)

func TestUserFrom_EmptyContext(t *testing.T) {
	if got := UserFrom(context.Background()); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestWithUser_Roundtrip(t *testing.T) {
	u := &AuthUser{Sub: "uuid-1", Email: "a@b.c", Role: "authenticated"}
	ctx := withUser(context.Background(), u)
	got := UserFrom(ctx)
	if got != u {
		t.Fatalf("expected same pointer, got %+v", got)
	}
}

func TestUserFrom_WrongType_ReturnsNil(t *testing.T) {
	// indirect verification that external packages cannot share the same key type:
	// inserting with a string key does not surface via UserFrom
	ctx := context.WithValue(context.Background(), "contextKey{}", &AuthUser{Sub: "x"})
	if got := UserFrom(ctx); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

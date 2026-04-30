package resolver_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

// mockUserRoleRepository satisfies repository.UserRoleRepository.
type mockUserRoleRepository struct {
	isAdmin bool
	err     error
}

func (m *mockUserRoleRepository) HasRole(_ context.Context, _, _ string) (bool, error) {
	return m.isAdmin, m.err
}

// newDictSrv builds a gqlgen handler backed by a resolver wired with the
// supplied UserRoleRepository mock.
func newDictSrv(roleRepo repository.UserRoleRepository) *handler.Server {
	authSvc := auth.NewService(roleRepo)
	r := &resolver.Resolver{AuthSvc: authSvc}
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// validateDictionaryQuery returns a JSON-encoded GraphQL query for validateDictionary.
func validateDictionaryQuery(payload string) string {
	return `{"query":"{ validateDictionary(input: { payload: \"` + payload + `\" }) { valid parsedWords { front back line } errors { line message } } }"}`
}

// TestValidateDictionary_NonAdmin verifies that a non-admin authenticated user
// receives a FORBIDDEN error.
func TestValidateDictionary_NonAdmin(t *testing.T) {
	t.Parallel()

	srv := newDictSrv(&mockUserRoleRepository{isAdmin: false})
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeForbidden, code)
	}
}

// TestValidateDictionary_AdminHappyPath verifies that an admin caller receives
// a valid result with parsed words when the payload contains a well-formed entry.
func TestValidateDictionary_AdminHappyPath(t *testing.T) {
	t.Parallel()

	srv := newDictSrv(&mockUserRoleRepository{isAdmin: true})
	// "apple<IDEOGRAPHIC SPACE><ri><n><go>" — written as UTF-8 escape sequences
	// to comply with the no-CJK-literal policy.
	// U+3000 IDEOGRAPHIC SPACE = \xe3\x80\x80
	// U+308A ri = \xe3\x82\x8a
	// U+3093 n  = \xe3\x82\x93
	// U+3054 go = \xe3\x81\x94
	raw := "apple\xe3\x80\x80\xe3\x82\x8a\xe3\x82\x93\xe3\x81\x94"
	payload := base64.StdEncoding.EncodeToString([]byte(raw))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["validateDictionary"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.validateDictionary, got nil; response: %v", resp)
	}
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
	words, _ := result["parsedWords"].([]any)
	if len(words) != 1 {
		t.Fatalf("expected 1 parsedWord, got %d", len(words))
	}
	errs, _ := result["errors"].([]any)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateDictionary_BadBase64 verifies that a malformed base64 payload
// surfaces a BAD_USER_INPUT error.
func TestValidateDictionary_BadBase64(t *testing.T) {
	t.Parallel()

	srv := newDictSrv(&mockUserRoleRepository{isAdmin: true})
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery("!!!not-base64!!!"))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeBadUserInput, code)
	}
}

// TestValidateDictionary_Unauthenticated verifies that an anonymous context
// surfaces an UNAUTHENTICATED error.
func TestValidateDictionary_Unauthenticated(t *testing.T) {
	t.Parallel()

	srv := newDictSrv(&mockUserRoleRepository{isAdmin: false})
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, context.Background(), validateDictionaryQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeUnauthenticated, code)
	}
}

// TestValidateDictionary_EmptyPayload verifies that an empty payload string
// surfaces a BAD_USER_INPUT error before any decoding or processing occurs.
func TestValidateDictionary_EmptyPayload(t *testing.T) {
	t.Parallel()

	srv := newDictSrv(&mockUserRoleRepository{isAdmin: true})
	// Send the empty string directly as the payload value (not base64 of "").
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(""))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeBadUserInput, code)
	}
}

// TestValidateDictionary_IsAdminError verifies that a database error returned
// by IsAdmin surfaces an INTERNAL error rather than leaking implementation
// details to the caller.
func TestValidateDictionary_IsAdminError(t *testing.T) {
	t.Parallel()

	srv := newDictSrv(&mockUserRoleRepository{err: errors.New("db down")})
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeInternal, code)
	}
}

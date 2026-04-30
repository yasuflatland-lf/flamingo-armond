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

// newDictOnlySrv builds a server with only AuthSvc wired; only the
// validateDictionary resolver is exercised here.
func newDictOnlySrv(roleRepo repository.UserRoleRepository) *handler.Server {
	r := &resolver.Resolver{AuthSvc: auth.NewService(roleRepo)}
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

func validateDictionaryQuery(payload string) string {
	return `{"query":"{ validateDictionary(input: { payload: \"` + payload + `\" }) { valid parsedWords { front back line } errors { line message } } }"}`
}

// TestValidateDictionary_NonAdmin verifies that a non-admin authenticated user
// receives a FORBIDDEN error.
func TestValidateDictionary_NonAdmin(t *testing.T) {
	t.Parallel()

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: false})
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

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: true})
	// "apple" + U+3000 ideographic space + hiragana "ringo" (apple), built
	// from UTF-8 byte literals so committed source stays ASCII-only.
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

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: true})
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery("!!!not-base64!!!"))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeBadUserInput, code)
	}
}

// TestValidateDictionary_NonAdminBadBase64 verifies that the admin check
// (FORBIDDEN) is performed before base64 validation (BAD_USER_INPUT).
// A non-admin user should receive FORBIDDEN even when the payload is malformed.
func TestValidateDictionary_NonAdminBadBase64(t *testing.T) {
	t.Parallel()

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: false})
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery("!!!not-base64!!!"))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeForbidden, code)
	}
}

// TestValidateDictionary_Unauthenticated verifies that an anonymous context
// surfaces an UNAUTHENTICATED error.
func TestValidateDictionary_Unauthenticated(t *testing.T) {
	t.Parallel()

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: false})
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

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: true})
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

	srv := newDictOnlySrv(&mockUserRoleRepository{err: errors.New("db down")})
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeInternal, code)
	}
}

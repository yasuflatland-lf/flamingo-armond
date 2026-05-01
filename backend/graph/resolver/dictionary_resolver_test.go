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
	"backend/internal/usecase"
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
	r := resolver.NewResolver(nil, nil, nil, nil, auth.NewService(roleRepo), nil, nil)
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

// TestValidateDictionary_IsAdminCancelled verifies that a context.Canceled
// error from IsAdmin surfaces a CANCELLED error rather than INTERNAL.
func TestValidateDictionary_IsAdminCancelled(t *testing.T) {
	t.Parallel()

	srv := newDictOnlySrv(&mockUserRoleRepository{err: context.Canceled})
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeCancelled) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeCancelled, code)
	}
}

// TestValidateDictionary_IsAdminDeadlineExceeded verifies that a
// context.DeadlineExceeded error from IsAdmin surfaces a CANCELLED error.
func TestValidateDictionary_IsAdminDeadlineExceeded(t *testing.T) {
	t.Parallel()

	srv := newDictOnlySrv(&mockUserRoleRepository{err: context.DeadlineExceeded})
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeCancelled) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeCancelled, code)
	}
}

// ---------------------------------------------------------------------------
// mockDictionaryUsecase — in-package stub for upsertDictionary resolver tests.
// ---------------------------------------------------------------------------

// mockDictionaryUsecase satisfies usecase.DictionaryUsecase. The returnOut
// field drives the happy-path result; the returnErr field, when non-nil, is
// returned instead so the error-propagation path can be exercised.
type mockDictionaryUsecase struct {
	returnOut usecase.UpsertDictionaryOutput
	returnErr error
}

func (m *mockDictionaryUsecase) Upsert(_ context.Context, _ usecase.UpsertDictionaryInput) (usecase.UpsertDictionaryOutput, error) {
	if m.returnErr != nil {
		return usecase.UpsertDictionaryOutput{}, m.returnErr
	}
	return m.returnOut, nil
}

// newUpsertDictSrv builds a server with the given DictionaryUsecase mock
// wired. AuthSvc is not needed for the upsertDictionary resolver because the
// usecase mock already encapsulates auth logic.
func newUpsertDictSrv(dictUC usecase.DictionaryUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, dictUC, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// upsertDictionaryMutation returns a JSON-encoded GraphQL mutation body for
// upsertDictionary. Both arguments are embedded literally so the caller must
// escape them if needed; for test use the values are always safe ASCII/base64.
func upsertDictionaryMutation(cardgroupID, payload string) string {
	return `{"query":"mutation { upsertDictionary(input: { cardgroupId: \"` + cardgroupID + `\", payload: \"` + payload + `\" }) { inserted updated errors { line message } } }"}`
}

// TestUpsertDictionary_ResolverHappyPath drives the full GraphQL transport with
// a mock DictionaryUsecase that returns a known UpsertDictionaryOutput. The
// test asserts the GraphQL response payload's inserted, updated, and errors[0]
// fields, which catches int64->int truncation regressions and nil-vs-empty
// errors slice mismatches.
func TestUpsertDictionary_ResolverHappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockDictionaryUsecase{
		returnOut: usecase.UpsertDictionaryOutput{
			Inserted: 7,
			Updated:  3,
			Errors: []usecase.DictionaryValidationError{
				{Line: 5, Message: "duplicate"},
			},
		},
	}
	srv := newUpsertDictSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), upsertDictionaryMutation("cg-1", payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["upsertDictionary"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.upsertDictionary, got nil; response: %v", resp)
	}

	// JSON numbers decode as float64 in Go's encoding/json.
	if got, _ := result["inserted"].(float64); int(got) != 7 {
		t.Fatalf("expected inserted=7, got %v", result["inserted"])
	}
	if got, _ := result["updated"].(float64); int(got) != 3 {
		t.Fatalf("expected updated=3, got %v", result["updated"])
	}

	errs, _ := result["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error entry, got %d: %v", len(errs), errs)
	}
	firstErr, _ := errs[0].(map[string]any)
	if line, _ := firstErr["line"].(float64); int(line) != 5 {
		t.Fatalf("expected errors[0].line=5, got %v", firstErr["line"])
	}
	if msg, _ := firstErr["message"].(string); msg != "duplicate" {
		t.Fatalf("expected errors[0].message=%q, got %q", "duplicate", msg)
	}
}

// TestUpsertDictionary_ResolverPropagatesForbidden verifies that when the
// DictionaryUsecase returns a FORBIDDEN gqlerror (e.g. non-admin caller), the
// resolver propagates it unchanged and the GraphQL response carries
// errors[0].extensions.code == "FORBIDDEN".
func TestUpsertDictionary_ResolverPropagatesForbidden(t *testing.T) {
	t.Parallel()

	mock := &mockDictionaryUsecase{
		returnErr: gqlerr.NewForbidden("admin role required"),
	}
	srv := newUpsertDictSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), upsertDictionaryMutation("cg-1", payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeForbidden, code)
	}
}

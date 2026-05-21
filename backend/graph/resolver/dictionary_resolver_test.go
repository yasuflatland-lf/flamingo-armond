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
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

// mockUserRoleRepository only implements HasRole — the narrow subset that
// auth.NewService requires. It does NOT satisfy the full repository.UserRoleRepository
// interface; use it only where HasRole is the only method called.
type mockUserRoleRepository struct {
	isAdmin bool
	err     error
}

func (m *mockUserRoleRepository) HasRole(_ context.Context, _ string, _ domain.RoleName) (bool, error) {
	return m.isAdmin, m.err
}

// hasRoleChecker is the narrow interface required by auth.NewService.
type hasRoleChecker interface {
	HasRole(ctx context.Context, userID string, roleName domain.RoleName) (bool, error)
}

// newDictOnlySrv builds a server with only DictionaryUsecase wired; only the
// validateDictionary resolver is exercised here.
func newDictOnlySrv(roleRepo hasRoleChecker) *handler.Server {
	authSvc := auth.NewService(roleRepo)
	dictUC := usecase.NewDictionaryUsecaseWithTx(usecase.NewAdminGate(authSvc), nil, nil, newDiscardLogger())
	r := resolver.NewResolver(nil, nil, nil, nil, authSvc, dictUC, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

func validateDictionaryQuery(payload string) string {
	return `{"query":"{ validateDictionary(input: { payload: \"` + payload + `\" }) { valid parsedWords { front back line } errors { line message kind snippet front back } } }"}`
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
	returnOut   usecase.UpsertDictionaryOutput
	returnErr   error
	validateOut usecase.ValidateDictionaryOutcome
	validateErr error
}

func (m *mockDictionaryUsecase) Upsert(_ context.Context, _ usecase.UpsertDictionaryInput) (usecase.UpsertDictionaryOutput, error) {
	if m.returnErr != nil {
		return usecase.UpsertDictionaryOutput{}, m.returnErr
	}
	return m.returnOut, nil
}

func (m *mockDictionaryUsecase) Validate(_ context.Context, _ string) (usecase.ValidateDictionaryOutcome, error) {
	if m.validateErr != nil {
		return usecase.ValidateDictionaryOutcome{}, m.validateErr
	}
	return m.validateOut, nil
}

// newUpsertDictSrv builds a server with the given DictionaryUsecase mock
// wired. AuthSvc is not needed for the upsertDictionary resolver because the
// usecase mock already encapsulates auth logic.
func newUpsertDictSrv(dictUC usecase.DictionaryUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, dictUC, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// upsertDictionaryMutation returns a JSON-encoded GraphQL mutation body for
// upsertDictionary. Both arguments are embedded literally so the caller must
// escape them if needed; for test use the values are always safe ASCII/base64.
// The errors selection set includes front and back so resolver mapping of those
// optional fields can be asserted.
func upsertDictionaryMutation(cardgroupID, payload string) string {
	return `{"query":"mutation { upsertDictionary(input: { cardgroupId: \"` + cardgroupID + `\", payload: \"` + payload + `\" }) { inserted updated errors { line message kind snippet front back } } }"}`
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
				{Line: 5, Message: "duplicate", Kind: usecase.DictErrKindHard},
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
	// A syntax-level error has no parsed front/back — the resolver must leave
	// both fields as null (i.e. the key is absent or nil in the JSON map).
	if v, present := firstErr["front"]; present && v != nil {
		t.Fatalf("expected errors[0].front=null for syntax error, got %v", v)
	}
	if v, present := firstErr["back"]; present && v != nil {
		t.Fatalf("expected errors[0].back=null for syntax error, got %v", v)
	}
}

// TestUpsertDictionary_ResolverMapsValidationErrorFrontBack verifies that when
// the DictionaryUsecase returns a DictionaryValidationError with non-empty
// Front and Back (the duplicate-front dedup path), the resolver maps them to
// non-nil *string pointers and the GraphQL response carries the values.
func TestUpsertDictionary_ResolverMapsValidationErrorFrontBack(t *testing.T) {
	t.Parallel()

	mock := &mockDictionaryUsecase{
		returnOut: usecase.UpsertDictionaryOutput{
			Inserted: 0,
			Updated:  1,
			Errors: []usecase.DictionaryValidationError{
				{
					Line:    3,
					Message: "duplicate front in payload (later occurrence wins)",
					Kind:    usecase.DictErrKindDuplicate,
					Front:   "apple",
					Back:    "fruit",
				},
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

	errs, _ := result["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error entry, got %d: %v", len(errs), errs)
	}
	entry, _ := errs[0].(map[string]any)
	if line, _ := entry["line"].(float64); int(line) != 3 {
		t.Fatalf("expected errors[0].line=3, got %v", entry["line"])
	}
	// front and back must be present and non-nil for a parsed duplicate row.
	front, _ := entry["front"].(string)
	if front != "apple" {
		t.Fatalf("expected errors[0].front=%q, got %v", "apple", entry["front"])
	}
	back, _ := entry["back"].(string)
	if back != "fruit" {
		t.Fatalf("expected errors[0].back=%q, got %v", "fruit", entry["back"])
	}
}

// TestUpsertDictionary_ResolverPropagatesForbidden verifies that when the
// DictionaryUsecase returns *ucerr.ForbiddenError (e.g. non-admin caller),
// the resolver wraps it via gqlerr.FromUsecaseError and the GraphQL
// response carries errors[0].extensions.code == "FORBIDDEN".
func TestUpsertDictionary_ResolverPropagatesForbidden(t *testing.T) {
	t.Parallel()

	mock := &mockDictionaryUsecase{
		returnErr: &ucerr.ForbiddenError{Message: "admin role required"},
	}
	srv := newUpsertDictSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), upsertDictionaryMutation("cg-1", payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeForbidden, code)
	}
}

// TestValidateDictionary_ResolverKindAndSnippet verifies that the
// validateDictionary resolver maps the kind and snippet fields from the
// usecase output to the GraphQL response wire shape. A lone-front payload
// ("orphan" with no back) must produce an error whose kind is "FRONT_ONLY"
// and whose snippet is the WORD token text (see
// TestUpsertDictionary_ResolverKindHardSnippetNull for the HARD/null-snippet
// pair).
func TestValidateDictionary_ResolverKindAndSnippet(t *testing.T) {
	t.Parallel()

	srv := newDictOnlySrv(&mockUserRoleRepository{isAdmin: true})

	// A lone front with no back produces one FRONT_ONLY skip error.
	// Snippet carries the WORD token text; front and back must be null.
	raw := "orphan"
	payload := base64.StdEncoding.EncodeToString([]byte(raw))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateDictionaryQuery(payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["validateDictionary"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.validateDictionary, got nil; response: %v", resp)
	}
	errs, _ := result["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %v", len(errs), errs)
	}
	entry, _ := errs[0].(map[string]any)

	// kind must be the FRONT_ONLY enum value.
	if kind, _ := entry["kind"].(string); kind != "FRONT_ONLY" {
		t.Fatalf("errors[0].kind = %q, want %q", kind, "FRONT_ONLY")
	}
	// snippet must be the WORD token text.
	if snip, _ := entry["snippet"].(string); snip != "orphan" {
		t.Fatalf("errors[0].snippet = %q, want %q", snip, "orphan")
	}
	// front and back must be null (skip errors do not carry dedupe token text).
	if v, present := entry["front"]; present && v != nil {
		t.Fatalf("errors[0].front = %v, want null for skip error", v)
	}
	if v, present := entry["back"]; present && v != nil {
		t.Fatalf("errors[0].back = %v, want null for skip error", v)
	}
}

// TestUpsertDictionary_ResolverKindHardSnippetNull verifies that the
// upsertDictionary resolver maps a hard payload-level error (HARD kind)
// with an empty snippet. The empty base64 payload triggers a BAD_USER_INPUT
// before the resolver is reached, so this test uses the mock usecase to
// inject a HARD-kind error directly.
func TestUpsertDictionary_ResolverKindHardSnippetNull(t *testing.T) {
	t.Parallel()

	mock := &mockDictionaryUsecase{
		returnOut: usecase.UpsertDictionaryOutput{
			Inserted: 0,
			Updated:  0,
			Errors: []usecase.DictionaryValidationError{
				{
					Line:    0,
					Message: "payload exceeds 1048576 bytes",
					Kind:    usecase.DictErrKindHard,
					Snippet: "",
				},
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
	errs, _ := result["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error entry, got %d: %v", len(errs), errs)
	}
	entry, _ := errs[0].(map[string]any)

	// kind must be the HARD enum value.
	if kind, _ := entry["kind"].(string); kind != "HARD" {
		t.Fatalf("errors[0].kind = %q, want %q", kind, "HARD")
	}
	// snippet must be null for hard errors.
	if v, present := entry["snippet"]; present && v != nil {
		t.Fatalf("errors[0].snippet = %v, want null for HARD error", v)
	}
}

// TestUpsertDictionary_ResolverKindDuplicateSnippetNull verifies the dedupe
// duplicate path: kind == "DUPLICATE", front and back are non-nil, snippet
// is null.
func TestUpsertDictionary_ResolverKindDuplicateSnippetNull(t *testing.T) {
	t.Parallel()

	mock := &mockDictionaryUsecase{
		returnOut: usecase.UpsertDictionaryOutput{
			Inserted: 1,
			Updated:  0,
			Errors: []usecase.DictionaryValidationError{
				{
					Line:    1,
					Message: "duplicate front in payload (later occurrence wins)",
					Kind:    usecase.DictErrKindDuplicate,
					Front:   "apple",
					Back:    "fruit",
					Snippet: "",
				},
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
	errs, _ := result["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error entry, got %d: %v", len(errs), errs)
	}
	entry, _ := errs[0].(map[string]any)

	// kind must be the DUPLICATE enum value.
	if kind, _ := entry["kind"].(string); kind != "DUPLICATE" {
		t.Fatalf("errors[0].kind = %q, want %q", kind, "DUPLICATE")
	}
	// front and back must be non-nil for a dedupe duplicate.
	if front, _ := entry["front"].(string); front != "apple" {
		t.Fatalf("errors[0].front = %v, want %q", entry["front"], "apple")
	}
	if back, _ := entry["back"].(string); back != "fruit" {
		t.Fatalf("errors[0].back = %v, want %q", entry["back"], "fruit")
	}
	// snippet must be null for the dedupe path.
	if v, present := entry["snippet"]; present && v != nil {
		t.Fatalf("errors[0].snippet = %v, want null for DUPLICATE error", v)
	}
}

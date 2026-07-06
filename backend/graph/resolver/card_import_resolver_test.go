package resolver_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

type cardImportResolverCardgroupRepo struct{}

func (cardImportResolverCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return nil, nil
}

// mockUserRoleRepository is a shared resolver-test stub for auth.Service callers.
type mockUserRoleRepository struct {
	isAdmin bool
	err     error
}

func (m *mockUserRoleRepository) HasRole(_ context.Context, _ string, _ domain.RoleName) (bool, error) {
	return m.isAdmin, m.err
}

// newCardImportOnlySrv builds a server with only CardImportUsecase wired; only
// the validateCardImport resolver is exercised here.
func newCardImportOnlySrv() *handler.Server {
	cardImportUC := usecase.NewCardImportUsecaseWithTx(cardImportResolverCardgroupRepo{}, nil, nil, newDiscardLogger())
	r := resolver.NewResolver(nil, nil, nil, nil, nil, cardImportUC, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

func validateCardImportQuery(payload string) string {
	return `{"query":"{ validateCardImport(input: { payload: \"` + payload + `\" }) { valid parsedCards { front back line } errors { line message kind snippet front back } } }"}`
}

// TestValidateCardImport_AuthenticatedHappyPath verifies that an authenticated
// caller receives a valid result with parsed cards when the payload contains a
// well-formed entry.
func TestValidateCardImport_AuthenticatedHappyPath(t *testing.T) {
	t.Parallel()

	srv := newCardImportOnlySrv()
	// "apple" + U+3000 ideographic space + hiragana "ringo" (apple), built
	// from UTF-8 byte literals so committed source stays ASCII-only.
	raw := "apple\xe3\x80\x80\xe3\x82\x8a\xe3\x82\x93\xe3\x81\x94"
	payload := base64.StdEncoding.EncodeToString([]byte(raw))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateCardImportQuery(payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["validateCardImport"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.validateCardImport, got nil; response: %v", resp)
	}
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
	words, _ := result["parsedCards"].([]any)
	if len(words) != 1 {
		t.Fatalf("expected 1 parsedCard, got %d", len(words))
	}
	errs, _ := result["errors"].([]any)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateCardImport_BadBase64 verifies that a malformed base64 payload
// surfaces a BAD_USER_INPUT error.
func TestValidateCardImport_BadBase64(t *testing.T) {
	t.Parallel()

	srv := newCardImportOnlySrv()
	resp := gqlRequest(t, srv, authedCtx("u1"), validateCardImportQuery("!!!not-base64!!!"))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeBadUserInput, code)
	}
}

// TestValidateCardImport_Unauthenticated verifies that an anonymous context
// surfaces an UNAUTHENTICATED error.
func TestValidateCardImport_Unauthenticated(t *testing.T) {
	t.Parallel()

	srv := newCardImportOnlySrv()
	payload := base64.StdEncoding.EncodeToString([]byte("apple"))
	resp := gqlRequest(t, srv, context.Background(), validateCardImportQuery(payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeUnauthenticated, code)
	}
}

// TestValidateCardImport_EmptyPayload verifies that an empty payload string
// surfaces a BAD_USER_INPUT error before any decoding or processing occurs.
func TestValidateCardImport_EmptyPayload(t *testing.T) {
	t.Parallel()

	srv := newCardImportOnlySrv()
	// Send the empty string directly as the payload value (not base64 of "").
	resp := gqlRequest(t, srv, authedCtx("u1"), validateCardImportQuery(""))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeBadUserInput, code)
	}
}

// ---------------------------------------------------------------------------
// mockCardImportUsecase — in-package stub for importCards resolver tests.
// ---------------------------------------------------------------------------

// mockCardImportUsecase satisfies usecase.CardImportUsecase. The returnOut
// field drives the happy-path result; the returnErr field, when non-nil, is
// returned instead so the error-propagation path can be exercised.
type mockCardImportUsecase struct {
	returnOut   usecase.ImportCardsOutput
	returnErr   error
	validateOut usecase.ValidateCardImportOutcome
	validateErr error
}

func (m *mockCardImportUsecase) Import(_ context.Context, _ usecase.ImportCardsInput) (usecase.ImportCardsOutput, error) {
	if m.returnErr != nil {
		return usecase.ImportCardsOutput{}, m.returnErr
	}
	return m.returnOut, nil
}

func (m *mockCardImportUsecase) Validate(_ context.Context, _ string) (usecase.ValidateCardImportOutcome, error) {
	if m.validateErr != nil {
		return usecase.ValidateCardImportOutcome{}, m.validateErr
	}
	return m.validateOut, nil
}

// newImportCardsSrv builds a server with the given CardImportUsecase mock
// wired. AuthSvc is not needed for the importCards resolver because the
// usecase mock already encapsulates auth logic.
func newImportCardsSrv(cardImportUC usecase.CardImportUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, cardImportUC, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// importCardsMutation returns a JSON-encoded GraphQL mutation body for
// importCards. Both arguments are embedded literally so the caller must
// escape them if needed; for test use the values are always safe ASCII/base64.
// The errors selection set includes front and back so resolver mapping of those
// optional fields can be asserted.
func importCardsMutation(cardgroupID, payload string) string {
	return `{"query":"mutation { importCards(input: { cardgroupId: \"` + cardgroupID + `\", payload: \"` + payload + `\" }) { inserted updated errors { line message kind snippet front back } } }"}`
}

// TestImportCards_ResolverHappyPath drives the full GraphQL transport with
// a mock CardImportUsecase that returns a known ImportCardsOutput. The
// test asserts the GraphQL response payload's inserted, updated, and errors[0]
// fields, which catches int64->int truncation regressions and nil-vs-empty
// errors slice mismatches.
func TestImportCards_ResolverHappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockCardImportUsecase{
		returnOut: usecase.ImportCardsOutput{
			Inserted: 7,
			Updated:  3,
			Errors: []usecase.CardImportError{
				{Line: 5, Message: "duplicate", Kind: usecase.CardImportErrKindHard},
			},
		},
	}
	srv := newImportCardsSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), importCardsMutation("cg-1", payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["importCards"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.importCards, got nil; response: %v", resp)
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

// TestImportCards_ResolverMapsValidationErrorFrontBack verifies that when
// the CardImportUsecase returns a CardImportError with non-empty
// Front and Back (the duplicate-front dedup path), the resolver maps them to
// non-nil *string pointers and the GraphQL response carries the values.
func TestImportCards_ResolverMapsValidationErrorFrontBack(t *testing.T) {
	t.Parallel()

	mock := &mockCardImportUsecase{
		returnOut: usecase.ImportCardsOutput{
			Inserted: 0,
			Updated:  1,
			Errors: []usecase.CardImportError{
				{
					Line:    3,
					Message: "duplicated front (apple) was overridden with the new back (fruit)",
					Kind:    usecase.CardImportErrKindDuplicate,
					Front:   "apple",
					Back:    "fruit",
				},
			},
		},
	}
	srv := newImportCardsSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), importCardsMutation("cg-1", payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["importCards"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.importCards, got nil; response: %v", resp)
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

// TestImportCards_ResolverPropagatesUnauthenticated verifies that when the
// CardImportUsecase rejects a caller, the resolver wraps it via
// gqlerr.FromUsecaseError and the GraphQL response carries
// errors[0].extensions.code == "UNAUTHENTICATED".
func TestImportCards_ResolverPropagatesUnauthenticated(t *testing.T) {
	t.Parallel()

	mock := &mockCardImportUsecase{
		returnErr: ucerr.ErrUnauthenticated,
	}
	srv := newImportCardsSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), importCardsMutation("cg-1", payload))

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected %s, got %q", gqlerr.CodeUnauthenticated, code)
	}
}

// TestValidateCardImport_ResolverKindAndSnippet verifies that the
// validateCardImport resolver maps the kind and snippet fields from the
// usecase output to the GraphQL response wire shape. A lone-front payload
// ("orphan" with no back) must produce an error whose kind is "FRONT_ONLY"
// and whose snippet is the WORD token text (see
// TestImportCards_ResolverKindHardSnippetNull for the HARD/null-snippet
// pair).
func TestValidateCardImport_ResolverKindAndSnippet(t *testing.T) {
	t.Parallel()

	srv := newCardImportOnlySrv()

	// A lone front with no back produces one FRONT_ONLY skip error.
	// Snippet carries the WORD token text; front and back must be null.
	raw := "orphan"
	payload := base64.StdEncoding.EncodeToString([]byte(raw))
	resp := gqlRequest(t, srv, authedCtx("u1"), validateCardImportQuery(payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["validateCardImport"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.validateCardImport, got nil; response: %v", resp)
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

// TestImportCards_ResolverKindHardSnippetNull verifies that the
// importCards resolver maps a hard payload-level error (HARD kind)
// with an empty snippet. The empty base64 payload triggers a BAD_USER_INPUT
// before the resolver is reached, so this test uses the mock usecase to
// inject a HARD-kind error directly.
func TestImportCards_ResolverKindHardSnippetNull(t *testing.T) {
	t.Parallel()

	mock := &mockCardImportUsecase{
		returnOut: usecase.ImportCardsOutput{
			Inserted: 0,
			Updated:  0,
			Errors: []usecase.CardImportError{
				{
					Line:    0,
					Message: "payload exceeds 1048576 bytes",
					Kind:    usecase.CardImportErrKindHard,
					Snippet: "",
				},
			},
		},
	}
	srv := newImportCardsSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), importCardsMutation("cg-1", payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["importCards"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.importCards, got nil; response: %v", resp)
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

// TestImportCards_ResolverKindDuplicateSnippetNull verifies the dedupe
// duplicate path: kind == "DUPLICATE", front and back are non-nil, snippet
// is null.
func TestImportCards_ResolverKindDuplicateSnippetNull(t *testing.T) {
	t.Parallel()

	mock := &mockCardImportUsecase{
		returnOut: usecase.ImportCardsOutput{
			Inserted: 1,
			Updated:  0,
			Errors: []usecase.CardImportError{
				{
					Line:    1,
					Message: "duplicated front (apple) was overridden with the new back (fruit)",
					Kind:    usecase.CardImportErrKindDuplicate,
					Front:   "apple",
					Back:    "fruit",
					Snippet: "",
				},
			},
		},
	}
	srv := newImportCardsSrv(mock)
	payload := base64.StdEncoding.EncodeToString([]byte("apple fruit"))
	resp := gqlRequest(t, srv, authedCtx("u1"), importCardsMutation("cg-1", payload))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected top-level errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["importCards"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.importCards, got nil; response: %v", resp)
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

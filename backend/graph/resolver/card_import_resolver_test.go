package resolver_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"gorm.io/gorm"

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

func (m *mockUserRoleRepository) HasRoleTx(_ context.Context, _ *gorm.DB, _ string, _ domain.RoleName) (bool, error) {
	return m.isAdmin, m.err
}

// newCardImportOnlySrv builds a server with only CardImportUsecase wired; only
// the validateCardImport resolver is exercised here.
func newCardImportOnlySrv() *handler.Server {
	cardImportUC := usecase.NewCardImportUsecaseWithTx(cardImportResolverCardgroupRepo{}, nil, nil)
	r := resolver.NewResolver(nil, nil, nil, nil, cardImportUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

func validateCardImportQuery(payload string) string {
	return `{"query":"{ validateCardImport(input: { payload: \"` + payload + `\" }) { valid parsedCards { front back line } errors { line message kind } } }"}`
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
// wired.
func newImportCardsSrv(cardImportUC usecase.CardImportUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, cardImportUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// importCardsMutation returns a JSON-encoded GraphQL mutation body for
// importCards. Both arguments are embedded literally so the caller must
// escape them if needed; for test use the values are always safe ASCII/base64.
func importCardsMutation(cardgroupID, payload string) string {
	return `{"query":"mutation { importCards(input: { cardgroupId: \"` + cardgroupID + `\", payload: \"` + payload + `\" }) { inserted updated errors { line message kind } } }"}`
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
}

// TestImportCards_ResolverMapsValidationErrorLine verifies that when the
// CardImportUsecase returns a CardImportError on the duplicate-front dedup
// path, the resolver carries its line number into the GraphQL response.
func TestImportCards_ResolverMapsValidationErrorLine(t *testing.T) {
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

// TestValidateCardImport_ResolverKind verifies that the validateCardImport
// resolver maps the kind field from the usecase output to the GraphQL
// response wire shape. A lone-front payload ("orphan" with no back) must
// produce an error whose kind is "FRONT_ONLY" (see
// TestImportCards_ResolverKindHard for the HARD counterpart).
func TestValidateCardImport_ResolverKind(t *testing.T) {
	t.Parallel()

	srv := newCardImportOnlySrv()

	// A lone front with no back produces one FRONT_ONLY skip error.
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
}

// TestImportCards_ResolverKindHard verifies that the importCards resolver maps
// a hard parser error to the HARD kind. Every whole-payload reject is a
// top-level BAD_USER_INPUT that never reaches the resolver's mapper, so this
// test uses the mock usecase to inject a HARD-kind error directly.
func TestImportCards_ResolverKindHard(t *testing.T) {
	t.Parallel()

	mock := &mockCardImportUsecase{
		returnOut: usecase.ImportCardsOutput{
			Inserted: 0,
			Updated:  0,
			Errors: []usecase.CardImportError{
				{
					Line:    0,
					Message: "unrecoverable parse failure",
					Kind:    usecase.CardImportErrKindHard,
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
}

// TestImportCards_ResolverKindDuplicate verifies that the importCards resolver
// maps a dedupe duplicate to the DUPLICATE kind.
func TestImportCards_ResolverKindDuplicate(t *testing.T) {
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
}

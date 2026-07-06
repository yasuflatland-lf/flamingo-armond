package resolver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// mockUserRepository satisfies usecase.UserRepository.
type mockUserRepository struct {
	findResult    *domain.User
	findErr       error
	updateResult  *domain.User
	updateErr     error
	capturedPatch repository.UserUpdate
	deleteAuthErr error
}

func (m *mockUserRepository) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return m.findResult, m.findErr
}

func (m *mockUserRepository) Update(_ context.Context, _ string, patch repository.UserUpdate) (*domain.User, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

func (m *mockUserRepository) DeleteAuthUser(_ context.Context, _ string) error {
	return m.deleteAuthErr
}

// ptr returns a pointer to s.
func ptr(s string) *string { return &s }

// dnPtr returns a *domain.DisplayName for the supplied string. Used by User
// fixture builders because Go does not allow taking the address of a conversion
// expression like &domain.DisplayName(s).
func dnPtr(s string) *domain.DisplayName {
	d := domain.DisplayName(s)
	return &d
}

// newServer builds a gqlgen handler.Server backed by a resolver that uses the
// given mock repository.
func newServer(mock *mockUserRepository) *handler.Server {
	uc := usecase.NewUserUsecase(mock, nil, nil, newDiscardLogger())
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// gqlRequest posts a GraphQL operation to the server with the given context,
// and returns a decoded response envelope.
func gqlRequest(t *testing.T, srv *handler.Server, ctx context.Context, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBufferString(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	var out map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// authedCtx returns a context with an authenticated user.
func authedCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

// errCode extracts response.errors[0].extensions.code.
func errCode(t *testing.T, resp map[string]any) string {
	t.Helper()
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("expected errors in response, got %v", resp)
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	code, _ := ext["code"].(string)
	return code
}

// meQuery is the GraphQL query used in Me tests.
const meQuery = `{"query":"{ me { id displayName bio avatarUrl } }"}`

// TestResolver_Me_Authenticated verifies that a context with an authenticated
// user returns the expected User shape.
func TestResolver_Me_Authenticated(t *testing.T) {
	t.Parallel()
	mock := &mockUserRepository{
		findResult: &domain.User{ID: "u1", DisplayName: dnPtr("Alice")},
	}
	srv := newServer(mock)
	resp := gqlRequest(t, srv, authedCtx("u1"), meQuery)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; full response: %v", resp)
	}
	if me["id"] != "u1" {
		t.Fatalf("expected id=u1, got %v", me["id"])
	}
	if me["displayName"] != "Alice" {
		t.Fatalf("expected displayName=Alice, got %v", me["displayName"])
	}
}

// TestResolver_Me_Anonymous verifies that an anonymous context causes
// response.errors[0].extensions.code == "UNAUTHENTICATED".
func TestResolver_Me_Anonymous(t *testing.T) {
	t.Parallel()
	mock := &mockUserRepository{}
	srv := newServer(mock)
	resp := gqlRequest(t, srv, context.Background(), meQuery)

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// updateProfileMutation returns a JSON-encoded GraphQL mutation body.
// The query selects across both union variants so tests can assert either
// the UpdateProfileSuccess or InputValidationError shape.
func updateProfileMutation(displayName string, bio *string) string {
	type vars struct {
		Input struct {
			DisplayName string  `json:"displayName"`
			Bio         *string `json:"bio,omitempty"`
		} `json:"input"`
	}
	v := vars{}
	v.Input.DisplayName = displayName
	v.Input.Bio = bio

	b, _ := json.Marshal(map[string]any{
		"query": `mutation($input: UpdateProfileInput!) {
			updateProfile(input: $input) {
				__typename
				... on UpdateProfileSuccess { user { id displayName bio } }
				... on InputValidationError { field message }
			}
		}`,
		"variables": v,
	})
	return string(b)
}

// TestResolver_UpdateProfile_BioVariants verifies how the resolver passes the
// bio field through to the repository layer. Each sub-test issues the union
// query and asserts the UpdateProfileSuccess.user.bio passthrough.
func TestResolver_UpdateProfile_BioVariants(t *testing.T) {
	t.Parallel()

	returned := &domain.User{ID: "u1", DisplayName: dnPtr("Alice")}

	cases := []struct {
		name          string
		bio           *string
		checkBioIsNil bool
		wantBio       *string
	}{
		{
			name:          "bio==nil leaves patch.Bio nil",
			bio:           nil,
			checkBioIsNil: true,
		},
		{
			name:    "bio==&\"\" explicit clear passes through",
			bio:     ptr(""),
			wantBio: ptr(""),
		},
		{
			name:    "bio==&\"hello\" passes through",
			bio:     ptr("hello"),
			wantBio: ptr("hello"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockUserRepository{updateResult: returned}
			srv := newServer(mock)

			body := updateProfileMutation("Alice", tc.bio)
			resp := gqlRequest(t, srv, authedCtx("u1"), body)

			if _, hasErrs := resp["errors"]; hasErrs {
				t.Fatalf("unexpected errors: %v", resp["errors"])
			}

			data, _ := resp["data"].(map[string]any)
			payload, _ := data["updateProfile"].(map[string]any)
			if payload == nil {
				t.Fatalf("expected data.updateProfile, got nil; response: %v", resp)
			}
			if payload["__typename"] != "UpdateProfileSuccess" {
				t.Fatalf("expected __typename=UpdateProfileSuccess, got %v", payload["__typename"])
			}

			if tc.checkBioIsNil {
				if mock.capturedPatch.Bio != nil {
					t.Fatalf("expected patch.Bio == nil, got %q", *mock.capturedPatch.Bio)
				}
			} else if tc.wantBio != nil {
				if mock.capturedPatch.Bio == nil {
					t.Fatalf("expected patch.Bio == %q, got nil", *tc.wantBio)
				}
				if *mock.capturedPatch.Bio != *tc.wantBio {
					t.Fatalf("expected patch.Bio=%q, got %q", *tc.wantBio, *mock.capturedPatch.Bio)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestResolver_UpdateProfile_* — standard four-case coverage
// ---------------------------------------------------------------------------

// TestResolver_UpdateProfile_HappyPath verifies that a successful profile
// update returns the UpdateProfileSuccess union variant with the updated user.
func TestResolver_UpdateProfile_HappyPath(t *testing.T) {
	t.Parallel()

	returned := &domain.User{ID: "u1", DisplayName: dnPtr("Alice")}
	mock := &mockUserRepository{updateResult: returned}
	srv := newServer(mock)

	resp := gqlRequest(t, srv, authedCtx("u1"), updateProfileMutation("Alice", nil))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateProfile"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateProfile, got nil; response: %v", resp)
	}
	if payload["__typename"] != "UpdateProfileSuccess" {
		t.Fatalf("expected __typename=UpdateProfileSuccess, got %v", payload["__typename"])
	}
	user, _ := payload["user"].(map[string]any)
	if user == nil {
		t.Fatalf("expected user in success payload, got nil; response: %v", resp)
	}
	if user["id"] != "u1" {
		t.Fatalf("expected user.id=u1, got %v", user["id"])
	}
}

// TestResolver_UpdateProfile_InputValidation verifies that a display name that
// fails validation is surfaced as the InputValidationError union variant
// (errors as data), not as a GraphQL protocol error.
func TestResolver_UpdateProfile_InputValidation(t *testing.T) {
	t.Parallel()

	mock := &mockUserRepository{}
	srv := newServer(mock)

	// An empty displayName fails the 1-50 char validation inside UserUsecase.
	resp := gqlRequest(t, srv, authedCtx("u1"), updateProfileMutation("", nil))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors (validation should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateProfile"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateProfile, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v", payload["__typename"])
	}
	if payload["field"] == nil || payload["field"] == "" {
		t.Fatalf("expected non-empty field in InputValidationError, got %v", payload["field"])
	}
}

// TestResolver_UpdateProfile_Unauthenticated verifies that an anonymous
// request is rejected with UNAUTHENTICATED via gqlerr.FromUsecaseError.
func TestResolver_UpdateProfile_Unauthenticated(t *testing.T) {
	t.Parallel()

	mock := &mockUserRepository{}
	srv := newServer(mock)

	resp := gqlRequest(t, srv, context.Background(), updateProfileMutation("Alice", nil))

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// TestResolver_UpdateProfile_NilVariant_ReturnsInternal covers the defensive
// guard in the resolver where the usecase returns an UpdateProfileOutcome with
// both User and Validation nil (a bug shape). Because UserUsecase is a concrete
// struct (not an interface), this guard is structurally unreachable through the
// real usecase; the guard is verified by constructing a zero-value outcome via
// the repository returning a nil user alongside a nil error.
//
// The repository Update mock returns nil, nil which causes UserUsecase to
// return UpdateProfileOutcome{} with User nil. The resolver then hits the
// nil-variant guard and returns INTERNAL.
func TestResolver_UpdateProfile_NilVariant_ReturnsInternal(t *testing.T) {
	t.Parallel()

	// updateResult is nil (zero value) — UserUsecase will wrap this and return
	// UpdateProfileOutcome{User: nil} with nil error, triggering the INTERNAL guard.
	mock := &mockUserRepository{updateResult: nil, updateErr: nil}
	srv := newServer(mock)

	resp := gqlRequest(t, srv, authedCtx("u1"), updateProfileMutation("Alice", nil))

	code := errCode(t, resp)
	if code != "INTERNAL" {
		t.Fatalf("expected INTERNAL, got %q; response: %v", code, resp)
	}
}

// newDeleteMyAccountSrv builds a server whose UserUsecase is wired with the
// admin-guard dependencies DeleteMyAccount needs: an AuthSvc reporting isAdmin
// for the caller and a roles repo reporting the global admin count.
func newDeleteMyAccountSrv(repo *mockUserRepository, isAdmin bool, adminCount int64) *handler.Server {
	authSvc := auth.NewService(&mockUserRoleRepository{isAdmin: isAdmin})
	uc := usecase.NewUserUsecase(repo, &mockRoleByUserIDRepo{adminCount: adminCount}, authSvc, newDiscardLogger())
	r := resolver.NewResolver(uc, nil, nil, nil, authSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

const deleteMyAccountMutation = `{"query":"mutation { deleteMyAccount }"}`

// TestUserResolver_DeleteMyAccount_Success verifies the mutation returns true
// when the usecase deletes the caller's account.
func TestUserResolver_DeleteMyAccount_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepository{}
	srv := newDeleteMyAccountSrv(repo, false, 0)
	resp := gqlRequest(t, srv, authedCtx("u1"), deleteMyAccountMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	if deleted, _ := data["deleteMyAccount"].(bool); !deleted {
		t.Fatalf("expected data.deleteMyAccount == true, got %v", data["deleteMyAccount"])
	}
}

// TestUserResolver_DeleteMyAccount_Unauthenticated verifies an anonymous caller
// receives UNAUTHENTICATED.
func TestUserResolver_DeleteMyAccount_Unauthenticated(t *testing.T) {
	t.Parallel()

	srv := newDeleteMyAccountSrv(&mockUserRepository{}, false, 0)
	resp := gqlRequest(t, srv, context.Background(), deleteMyAccountMutation)

	if code := errCode(t, resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; response: %v", code, resp)
	}
}

// TestUserResolver_DeleteMyAccount_LastAdminForbidden verifies the sole admin
// cannot delete their own account via the self-service path.
func TestUserResolver_DeleteMyAccount_LastAdminForbidden(t *testing.T) {
	t.Parallel()

	// Caller is an admin and is the only admin (count == 1) → forbidden.
	srv := newDeleteMyAccountSrv(&mockUserRepository{}, true, 1)
	resp := gqlRequest(t, srv, authedCtx("u1"), deleteMyAccountMutation)

	if code := errCode(t, resp); code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

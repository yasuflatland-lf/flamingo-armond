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
}

func (m *mockUserRepository) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return m.findResult, m.findErr
}

func (m *mockUserRepository) Update(_ context.Context, _ string, patch repository.UserUpdate) (*domain.User, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

// ptr returns a pointer to s.
func ptr(s string) *string { return &s }

// newServer builds a gqlgen handler.Server backed by a resolver that uses the
// given mock repository.
func newServer(mock *mockUserRepository) *handler.Server {
	uc := usecase.NewUserUsecase(mock)
	r := &resolver.Resolver{User: uc}
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
	displayName := "Alice"
	mock := &mockUserRepository{
		findResult: &domain.User{ID: "u1", DisplayName: &displayName},
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
			updateProfile(input: $input) { user { id displayName bio } }
		}`,
		"variables": v,
	})
	return string(b)
}

// TestResolver_UpdateProfile_BioVariants verifies how the resolver passes the
// bio field through to the repository layer.
func TestResolver_UpdateProfile_BioVariants(t *testing.T) {
	t.Parallel()

	returned := &domain.User{ID: "u1", DisplayName: ptr("Alice")}

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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockUserRepository{updateResult: returned}
			srv := newServer(mock)

			body := updateProfileMutation("Alice", tc.bio)
			resp := gqlRequest(t, srv, authedCtx("u1"), body)

			if _, hasErrs := resp["errors"]; hasErrs {
				t.Fatalf("unexpected errors: %v", resp["errors"])
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

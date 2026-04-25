package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// mockProfileRepository is a hand-written test double for ProfileRepository.
type mockProfileRepository struct {
	findResult    *domain.Profile
	findErr       error
	updateResult  *domain.Profile
	updateErr     error
	capturedPatch repository.ProfileUpdate
}

func (m *mockProfileRepository) FindByID(_ context.Context, _ string) (*domain.Profile, error) {
	return m.findResult, m.findErr
}

func (m *mockProfileRepository) Update(_ context.Context, _ string, patch repository.ProfileUpdate) (*domain.Profile, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

// authedCtx returns a context carrying an authenticated user with the given sub.
func authedCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

// anonCtx returns a context with no authenticated user.
func anonCtx() context.Context {
	return context.Background()
}

// ptr returns a pointer to s — convenience for test literals.
func ptr(s string) *string { return &s }

// assertGQLCode asserts that err is a *gqlerror.Error with the given extensions code.
func assertGQLCode(t *testing.T, err error, code string) {
	t.Helper()
	var gqlErr *gqlerror.Error
	if !errors.As(err, &gqlErr) {
		t.Fatalf("expected *gqlerror.Error, got %T: %v", err, err)
	}
	got, _ := gqlErr.Extensions["code"].(string)
	if got != code {
		t.Fatalf("expected extensions.code=%q, got %q", code, got)
	}
}

// --- Me tests ---

func TestProfileUsecase_Me(t *testing.T) {
	t.Parallel()

	alice := ptr("Alice")
	cases := []struct {
		name       string
		ctx        context.Context
		findResult *domain.Profile
		findErr    error
		wantErr    string // gqlerror extensions.code, empty = no error
		wantID     string
	}{
		{
			name:    "unauthenticated returns UNAUTHENTICATED",
			ctx:     anonCtx(),
			wantErr: "UNAUTHENTICATED",
		},
		{
			name:       "normal returns profile from repo",
			ctx:        authedCtx("u1"),
			findResult: &domain.Profile{ID: "u1", DisplayName: alice},
			wantID:     "u1",
		},
		{
			name:    "ErrNotFound returns empty profile with no error",
			ctx:     authedCtx("u1"),
			findErr: repository.ErrNotFound,
			wantID:  "u1",
		},
		{
			name:    "non-ErrNotFound DB error returns INTERNAL",
			ctx:     authedCtx("u1"),
			findErr: errors.New("db died"),
			wantErr: "INTERNAL",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &mockProfileRepository{findResult: tc.findResult, findErr: tc.findErr}
			uc := NewProfileUsecase(repo)

			p, err := uc.Me(tc.ctx)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				assertGQLCode(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p == nil {
				t.Fatal("expected profile, got nil")
			}
			if p.ID != tc.wantID {
				t.Fatalf("expected profile.ID=%q, got %q", tc.wantID, p.ID)
			}
		})
	}
}

// --- UpdateProfile tests ---

func TestProfileUsecase_UpdateProfile(t *testing.T) {
	t.Parallel()

	returned := &domain.Profile{ID: "u1", DisplayName: ptr("Alice")}

	// familyEmoji is a ZWJ sequence that counts as 1 grapheme cluster.
	familyEmoji := "👨‍👩‍👧‍👦"
	// familyEmoji3 is a 3-person ZWJ family that counts as 1 grapheme cluster.
	familyEmoji3 := "👨‍👩‍👧"

	cases := []struct {
		name          string
		ctx           context.Context
		input         UpdateProfileInput
		repoResult    *domain.Profile
		repoErr       error
		wantErrCode   string // non-empty = expect gqlerror with this code
		wantErrField  string // non-empty = check extensions.field
		wantRepoName  *string
		wantRepoBio   *string
		checkBioIsNil bool
	}{
		{
			name:        "unauthenticated returns UNAUTHENTICATED",
			ctx:         anonCtx(),
			input:       UpdateProfileInput{DisplayName: "Alice"},
			wantErrCode: "UNAUTHENTICATED",
		},
		{
			name:         "empty displayName returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: ""},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:         "51-rune displayName returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: strings.Repeat("a", 51)},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:       "50-rune displayName with emoji passes",
			ctx:        authedCtx("u1"),
			input:      UpdateProfileInput{DisplayName: strings.Repeat("🦩", 50)},
			repoResult: returned,
			// grapheme count == 50 → valid
			wantRepoName: ptr(strings.Repeat("🦩", 50)),
		},
		{
			name:          "bio==nil passes through as nil to repo",
			ctx:           authedCtx("u1"),
			input:         UpdateProfileInput{DisplayName: "Alice", Bio: nil},
			repoResult:    returned,
			wantRepoName:  ptr("Alice"),
			checkBioIsNil: true,
		},
		{
			name:         "bio==&\"\" explicit clear passes through",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: "Alice", Bio: ptr("")},
			repoResult:   returned,
			wantRepoName: ptr("Alice"),
			wantRepoBio:  ptr(""),
		},
		{
			name:         "bio==&\"hello\" passes through",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: "Alice", Bio: ptr("hello")},
			repoResult:   returned,
			wantRepoName: ptr("Alice"),
			wantRepoBio:  ptr("hello"),
		},
		{
			name:         "displayName with surrounding whitespace is trimmed before repo",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: "  Alice  "},
			repoResult:   returned,
			wantRepoName: ptr("Alice"),
		},
		{
			name:        "repo error wrapped as INTERNAL",
			ctx:         authedCtx("u1"),
			input:       UpdateProfileInput{DisplayName: "Alice"},
			repoErr:     fmt.Errorf("db exploded"),
			wantErrCode: "INTERNAL",
		},
		// --- grapheme boundary tests ---
		{
			name:         "displayName_max_ok: 50 ASCII graphemes passes",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: strings.Repeat("a", 50)},
			repoResult:   returned,
			wantRepoName: ptr(strings.Repeat("a", 50)),
		},
		{
			name:         "displayName_over_max: 51 ASCII graphemes returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: strings.Repeat("a", 51)},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:         "displayName_emoji_zwj_50: 50 ZWJ family graphemes passes",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: strings.Repeat(familyEmoji, 50)},
			repoResult:   returned,
			wantRepoName: ptr(strings.Repeat(familyEmoji, 50)),
		},
		{
			name:         "displayName_emoji_zwj_51: 51 ZWJ family graphemes returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: strings.Repeat(familyEmoji, 51)},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:         "bio_max_500_ok: 500 ASCII graphemes bio passes",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: "valid", Bio: ptr(strings.Repeat("b", 500))},
			repoResult:   returned,
			wantRepoName: ptr("valid"),
			wantRepoBio:  ptr(strings.Repeat("b", 500)),
		},
		{
			name:         "bio_over_500: 501 ASCII graphemes bio returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: "valid", Bio: ptr(strings.Repeat("b", 501))},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "bio",
		},
		{
			name:         "bio_emoji_zwj_500: 500 ZWJ family graphemes bio passes",
			ctx:          authedCtx("u1"),
			input:        UpdateProfileInput{DisplayName: "valid", Bio: ptr(strings.Repeat(familyEmoji3, 500))},
			repoResult:   returned,
			wantRepoName: ptr("valid"),
			wantRepoBio:  ptr(strings.Repeat(familyEmoji3, 500)),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &mockProfileRepository{updateResult: tc.repoResult, updateErr: tc.repoErr}
			uc := NewProfileUsecase(repo)

			p, err := uc.UpdateProfile(tc.ctx, tc.input)

			// Case: expect a specific gqlerror code
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				assertGQLCode(t, err, tc.wantErrCode)
				if tc.wantErrField != "" {
					var gqlErr *gqlerror.Error
					errors.As(err, &gqlErr)
					got, _ := gqlErr.Extensions["field"].(string)
					if got != tc.wantErrField {
						t.Fatalf("expected extensions.field=%q, got %q", tc.wantErrField, got)
					}
				}
				return
			}

			// Case: success
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p == nil {
				t.Fatal("expected profile, got nil")
			}

			if tc.wantRepoName != nil {
				if repo.capturedPatch.DisplayName == nil {
					t.Fatal("expected DisplayName in patch, got nil")
				}
				if *repo.capturedPatch.DisplayName != *tc.wantRepoName {
					t.Fatalf("expected repo displayName=%q, got %q", *tc.wantRepoName, *repo.capturedPatch.DisplayName)
				}
			}

			if tc.checkBioIsNil {
				if repo.capturedPatch.Bio != nil {
					t.Fatalf("expected Bio==nil in patch, got %v", *repo.capturedPatch.Bio)
				}
			} else if tc.wantRepoBio != nil {
				if repo.capturedPatch.Bio == nil {
					t.Fatalf("expected Bio==%q in patch, got nil", *tc.wantRepoBio)
				}
				if *repo.capturedPatch.Bio != *tc.wantRepoBio {
					t.Fatalf("expected repo bio=%q, got %q", *tc.wantRepoBio, *repo.capturedPatch.Bio)
				}
			}
		})
	}
}

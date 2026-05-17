package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

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

func authedCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

func anonCtx() context.Context {
	return context.Background()
}

func ptr(s string) *string { return &s }

// --- Me tests ---

func TestUserUsecase_Me(t *testing.T) {
	t.Parallel()

	alice := ptr("Alice")
	cases := []struct {
		name       string
		ctx        context.Context
		findResult *domain.User
		findErr    error
		wantErr    string // expected outcome label: "UNAUTHENTICATED" (sentinel) | "INTERNAL" (eris-wrapped chain) | "" (no error)
		wantID     string
	}{
		{
			name:    "unauthenticated returns UNAUTHENTICATED",
			ctx:     anonCtx(),
			wantErr: "UNAUTHENTICATED",
		},
		{
			name:       "normal returns user from repo",
			ctx:        authedCtx("u1"),
			findResult: &domain.User{ID: "u1", DisplayName: alice},
			wantID:     "u1",
		},
		{
			name:    "ErrNotFound returns empty user with no error",
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &mockUserRepository{findResult: tc.findResult, findErr: tc.findErr}
			uc := NewUserUsecase(repo, newTestLogger())

			p, err := uc.Me(tc.ctx)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				switch tc.wantErr {
				case "UNAUTHENTICATED":
					assertUnauthenticated(t, err)
				case "INTERNAL":
					assertInternalChain(t, err, "usecase: Me: find user by ID")
				default:
					t.Fatalf("unhandled wantErr code %q in test", tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p == nil {
				t.Fatal("expected user, got nil")
			}
			if p.ID != tc.wantID {
				t.Fatalf("expected user.ID=%q, got %q", tc.wantID, p.ID)
			}
		})
	}
}

// --- UpdateUser tests ---

func TestUserUsecase_UpdateUser(t *testing.T) {
	t.Parallel()

	returned := &domain.User{ID: "u1", DisplayName: ptr("Alice")}

	// familyEmoji is a ZWJ sequence that counts as 1 grapheme cluster.
	familyEmoji := "👨‍👩‍👧‍👦"
	// familyEmoji3 is a 3-person ZWJ family that counts as 1 grapheme cluster.
	familyEmoji3 := "👨‍👩‍👧"

	cases := []struct {
		name          string
		ctx           context.Context
		input         UpdateUserInput
		repoResult    *domain.User
		repoErr       error
		wantErrCode   string // expected outcome label: "UNAUTHENTICATED" (sentinel) | "BAD_USER_INPUT" (*ucerr.ValidationError) | "INTERNAL" (eris-wrapped chain) | "" (no error)
		wantErrField  string // non-empty = check *ucerr.ValidationError.Field
		wantRepoName  *string
		wantRepoBio   *string
		checkBioIsNil bool
	}{
		{
			name:        "unauthenticated returns UNAUTHENTICATED",
			ctx:         anonCtx(),
			input:       UpdateUserInput{DisplayName: "Alice"},
			wantErrCode: "UNAUTHENTICATED",
		},
		{
			name:         "empty displayName returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: ""},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:         "51-rune displayName returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: strings.Repeat("a", 51)},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:       "50-rune displayName with emoji passes",
			ctx:        authedCtx("u1"),
			input:      UpdateUserInput{DisplayName: strings.Repeat("🦩", 50)},
			repoResult: returned,
			// grapheme count == 50 → valid
			wantRepoName: ptr(strings.Repeat("🦩", 50)),
		},
		{
			name:          "bio==nil passes through as nil to repo",
			ctx:           authedCtx("u1"),
			input:         UpdateUserInput{DisplayName: "Alice", Bio: nil},
			repoResult:    returned,
			wantRepoName:  ptr("Alice"),
			checkBioIsNil: true,
		},
		{
			name:         "bio==&\"\" explicit clear passes through",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: "Alice", Bio: ptr("")},
			repoResult:   returned,
			wantRepoName: ptr("Alice"),
			wantRepoBio:  ptr(""),
		},
		{
			name:         "bio==&\"hello\" passes through",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: "Alice", Bio: ptr("hello")},
			repoResult:   returned,
			wantRepoName: ptr("Alice"),
			wantRepoBio:  ptr("hello"),
		},
		{
			name:         "displayName with surrounding whitespace is trimmed before repo",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: "  Alice  "},
			repoResult:   returned,
			wantRepoName: ptr("Alice"),
		},
		{
			name:        "repo error wrapped as INTERNAL",
			ctx:         authedCtx("u1"),
			input:       UpdateUserInput{DisplayName: "Alice"},
			repoErr:     fmt.Errorf("db exploded"),
			wantErrCode: "INTERNAL",
		},
		// --- grapheme boundary tests ---
		{
			name:         "displayName_max_ok: 50 ASCII graphemes passes",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: strings.Repeat("a", 50)},
			repoResult:   returned,
			wantRepoName: ptr(strings.Repeat("a", 50)),
		},
		{
			name:         "displayName_over_max: 51 ASCII graphemes returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: strings.Repeat("a", 51)},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:         "displayName_emoji_zwj_50: 50 ZWJ family graphemes passes",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: strings.Repeat(familyEmoji, 50)},
			repoResult:   returned,
			wantRepoName: ptr(strings.Repeat(familyEmoji, 50)),
		},
		{
			name:         "displayName_emoji_zwj_51: 51 ZWJ family graphemes returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: strings.Repeat(familyEmoji, 51)},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "displayName",
		},
		{
			name:         "bio_max_500_ok: 500 ASCII graphemes bio passes",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: "valid", Bio: ptr(strings.Repeat("b", 500))},
			repoResult:   returned,
			wantRepoName: ptr("valid"),
			wantRepoBio:  ptr(strings.Repeat("b", 500)),
		},
		{
			name:         "bio_over_500: 501 ASCII graphemes bio returns BAD_USER_INPUT",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: "valid", Bio: ptr(strings.Repeat("b", 501))},
			wantErrCode:  "BAD_USER_INPUT",
			wantErrField: "bio",
		},
		{
			name:         "bio_emoji_zwj_500: 500 ZWJ family graphemes bio passes",
			ctx:          authedCtx("u1"),
			input:        UpdateUserInput{DisplayName: "valid", Bio: ptr(strings.Repeat(familyEmoji3, 500))},
			repoResult:   returned,
			wantRepoName: ptr("valid"),
			wantRepoBio:  ptr(strings.Repeat(familyEmoji3, 500)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &mockUserRepository{updateResult: tc.repoResult, updateErr: tc.repoErr}
			uc := NewUserUsecase(repo, newTestLogger())

			outcome, err := uc.UpdateUser(tc.ctx, tc.input)

			if tc.wantErrCode != "" {
				switch tc.wantErrCode {
				case "UNAUTHENTICATED":
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					assertUnauthenticated(t, err)
				case "BAD_USER_INPUT":
					// Validation failures are promoted to the outcome variant (nil error).
					if err != nil {
						t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
					}
					if outcome.Validation == nil {
						t.Fatal("expected non-nil Validation on BAD_USER_INPUT case")
					}
					if tc.wantErrField != "" && outcome.Validation.Field != tc.wantErrField {
						t.Fatalf("expected Validation.Field=%q, got %q", tc.wantErrField, outcome.Validation.Field)
					}
				case "INTERNAL":
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					assertInternalChain(t, err, "usecase: UpdateUser: update user")
				default:
					t.Fatalf("unhandled wantErrCode %q in test", tc.wantErrCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if outcome.User == nil {
				t.Fatal("expected non-nil User on success")
			}
			if outcome.Validation != nil {
				t.Fatalf("expected nil Validation on success, got %+v", outcome.Validation)
			}

			if tc.wantRepoName != nil {
				if repo.capturedPatch.DisplayName == nil {
					t.Fatal("expected DisplayName in patch, got nil")
				}
				if *repo.capturedPatch.DisplayName != *tc.wantRepoName {
					t.Fatalf("expected repo displayName=%q, got %q", *tc.wantRepoName, *repo.capturedPatch.DisplayName)
				}
			}

			switch {
			case tc.checkBioIsNil:
				if repo.capturedPatch.Bio != nil {
					t.Fatalf("expected Bio==nil in patch, got %v", *repo.capturedPatch.Bio)
				}
			case tc.wantRepoBio != nil:
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

func TestUserUsecase_UpdateUser_SuccessVariant(t *testing.T) {
	t.Parallel()

	returned := &domain.User{ID: "u1", DisplayName: ptr("Alice")}
	repo := &mockUserRepository{updateResult: returned}
	uc := NewUserUsecase(repo, newTestLogger())

	outcome, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{DisplayName: "Alice"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.User == nil {
		t.Fatal("expected non-nil User on success")
	}
	if outcome.Validation != nil {
		t.Fatalf("expected nil Validation on success, got %+v", outcome.Validation)
	}
}

func TestUserUsecase_UpdateUser_ValidationVariant_DisplayName(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepository{}
	uc := NewUserUsecase(repo, newTestLogger())

	outcome, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{DisplayName: ""})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.User != nil {
		t.Fatal("expected nil User on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on empty displayName")
	}
	if outcome.Validation.Field != "displayName" {
		t.Fatalf("expected Validation.Field=%q, got %q", "displayName", outcome.Validation.Field)
	}
	if repo.capturedPatch.DisplayName != nil {
		t.Fatal("repository.Update must not be called on validation failure")
	}
}

func TestUserUsecase_UpdateUser_ValidationVariant_Bio(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepository{}
	uc := NewUserUsecase(repo, newTestLogger())

	outcome, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{
		DisplayName: "Alice",
		Bio:         ptr(strings.Repeat("b", 501)),
	})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.User != nil {
		t.Fatal("expected nil User on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on oversized bio")
	}
	if outcome.Validation.Field != "bio" {
		t.Fatalf("expected Validation.Field=%q, got %q", "bio", outcome.Validation.Field)
	}
	if repo.capturedPatch.DisplayName != nil {
		t.Fatal("repository.Update must not be called on validation failure")
	}
}

func TestUserUsecase_UpdateUser_RepoError_InfraChannel(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepository{updateErr: errors.New("db: storage failure")}
	uc := NewUserUsecase(repo, newTestLogger())

	_, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{DisplayName: "Alice"})

	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	assertInternalChain(t, err, "usecase: UpdateUser: update user")
}

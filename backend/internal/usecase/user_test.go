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

	deleteAuthErr    error
	deleteAuthCalls  int
	lastDeleteAuthID string
}

func (m *mockUserRepository) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return m.findResult, m.findErr
}

func (m *mockUserRepository) Update(_ context.Context, _ string, patch repository.UserUpdate) (*domain.User, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

func (m *mockUserRepository) DeleteAuthUser(_ context.Context, id string) error {
	m.deleteAuthCalls++
	m.lastDeleteAuthID = id
	return m.deleteAuthErr
}

type mockUserRolesRepository struct {
	roles  []*domain.Role
	err    error
	calls  int
	userID string

	adminCount     int64
	countAdminsErr error
}

func (m *mockUserRolesRepository) ListByUser(_ context.Context, userID string) ([]*domain.Role, error) {
	m.calls++
	m.userID = userID
	return m.roles, m.err
}

func (m *mockUserRolesRepository) CountAdmins(_ context.Context) (int64, error) {
	return m.adminCount, m.countAdminsErr
}

func authedCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

func anonCtx() context.Context {
	return context.Background()
}

func ptr(s string) *string { return &s }

// dnPtr returns a *domain.DisplayName for the supplied string. Used by User
// fixture builders because Go does not allow taking the address of a conversion
// expression like &domain.DisplayName(s).
func dnPtr(s string) *domain.DisplayName {
	d := domain.DisplayName(s)
	return &d
}

// --- Me tests ---

func TestUserUsecase_Me(t *testing.T) {
	t.Parallel()

	alice := dnPtr("Alice")
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
			uc := NewUserUsecase(repo, nil, nil, newTestLogger())

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

func TestUserUsecase_RolesFor(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{{ID: "r-general", Name: domain.RoleName("general")}}

	t.Run("self can read roles without admin check", func(t *testing.T) {
		t.Parallel()
		roleRepo := &mockUserRolesRepository{roles: roles}
		authChk := &mockAdminChecker{isAdmin: false}
		uc := NewUserUsecase(&mockUserRepository{}, roleRepo, authChk, newTestLogger())

		got, err := uc.RolesFor(authedCtx("u-self"), "u-self")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].ID != "r-general" {
			t.Fatalf("unexpected roles: %+v", got)
		}
		if authChk.calls != 0 {
			t.Fatalf("expected 0 admin checks for self, got %d", authChk.calls)
		}
		if roleRepo.calls != 1 || roleRepo.userID != "u-self" {
			t.Fatalf("role repo calls/userID = %d/%q, want 1/u-self", roleRepo.calls, roleRepo.userID)
		}
	})

	t.Run("admin can read another user roles", func(t *testing.T) {
		t.Parallel()
		roleRepo := &mockUserRolesRepository{roles: roles}
		authChk := &mockAdminChecker{isAdmin: true}
		uc := NewUserUsecase(&mockUserRepository{}, roleRepo, authChk, newTestLogger())

		got, err := uc.RolesFor(authedCtx("admin-1"), "u-target")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 role, got %+v", got)
		}
		if authChk.calls != 1 {
			t.Fatalf("expected 1 admin check, got %d", authChk.calls)
		}
		if roleRepo.userID != "u-target" {
			t.Fatalf("expected roles for u-target, got %q", roleRepo.userID)
		}
	})

	t.Run("non-admin cannot read another user roles", func(t *testing.T) {
		t.Parallel()
		roleRepo := &mockUserRolesRepository{roles: roles}
		authChk := &mockAdminChecker{isAdmin: false}
		uc := NewUserUsecase(&mockUserRepository{}, roleRepo, authChk, newTestLogger())

		_, err := uc.RolesFor(authedCtx("user-1"), "u-target")

		assertForbidden(t, err, "admin only")
		if roleRepo.calls != 0 {
			t.Fatalf("expected 0 role repo calls on forbidden, got %d", roleRepo.calls)
		}
	})

	t.Run("anonymous", func(t *testing.T) {
		t.Parallel()
		roleRepo := &mockUserRolesRepository{roles: roles}
		authChk := &mockAdminChecker{isAdmin: true}
		uc := NewUserUsecase(&mockUserRepository{}, roleRepo, authChk, newTestLogger())

		_, err := uc.RolesFor(anonCtx(), "u-target")

		assertUnauthenticated(t, err)
		if authChk.calls != 0 || roleRepo.calls != 0 {
			t.Fatalf("expected no downstream calls, got auth=%d roles=%d", authChk.calls, roleRepo.calls)
		}
	})

	t.Run("admin check error", func(t *testing.T) {
		t.Parallel()
		roleRepo := &mockUserRolesRepository{roles: roles}
		authChk := &mockAdminChecker{err: errors.New("db down")}
		uc := NewUserUsecase(&mockUserRepository{}, roleRepo, authChk, newTestLogger())

		_, err := uc.RolesFor(authedCtx("admin-1"), "u-target")

		assertInternalChain(t, err, "usecase: user roles: check admin")
	})

	t.Run("roles repo error", func(t *testing.T) {
		t.Parallel()
		roleRepo := &mockUserRolesRepository{err: errors.New("roles db down")}
		authChk := &mockAdminChecker{isAdmin: true}
		uc := NewUserUsecase(&mockUserRepository{}, roleRepo, authChk, newTestLogger())

		_, err := uc.RolesFor(authedCtx("admin-1"), "u-target")

		assertInternalChain(t, err, "usecase: user roles: list by user")
	})
}

// --- UpdateUser tests ---

func TestUserUsecase_UpdateUser(t *testing.T) {
	t.Parallel()

	returned := &domain.User{ID: "u1", DisplayName: dnPtr("Alice")}

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
			uc := NewUserUsecase(repo, nil, nil, newTestLogger())

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

	returned := &domain.User{ID: "u1", DisplayName: dnPtr("Alice")}
	repo := &mockUserRepository{updateResult: returned}
	uc := NewUserUsecase(repo, nil, nil, newTestLogger())

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
	uc := NewUserUsecase(repo, nil, nil, newTestLogger())

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
	uc := NewUserUsecase(repo, nil, nil, newTestLogger())

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
	uc := NewUserUsecase(repo, nil, nil, newTestLogger())

	_, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{DisplayName: "Alice"})

	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	assertInternalChain(t, err, "usecase: UpdateUser: update user")
}

// TestUserUsecase_DeleteMyAccount exercises the self-service account-deletion
// guard chain and the happy path.
func TestUserUsecase_DeleteMyAccount(t *testing.T) {
	t.Parallel()

	const caller = "user-1"

	t.Run("unauthenticated", func(t *testing.T) {
		t.Parallel()
		uc := NewUserUsecase(&mockUserRepository{}, &mockUserRolesRepository{}, &mockAdminChecker{}, newTestLogger())
		assertUnauthenticated(t, uc.DeleteMyAccount(anonCtx()))
	})

	t.Run("non-admin caller is deleted without consulting the admin count", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		// adminCount is a blocking value to prove it is never consulted for non-admins.
		roles := &mockUserRolesRepository{adminCount: 1}
		uc := NewUserUsecase(repo, roles, &mockAdminChecker{isAdmin: false}, newTestLogger())

		if err := uc.DeleteMyAccount(authedCtx(caller)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.deleteAuthCalls != 1 || repo.lastDeleteAuthID != caller {
			t.Fatalf("DeleteAuthUser: calls=%d id=%q, want 1 and %q", repo.deleteAuthCalls, repo.lastDeleteAuthID, caller)
		}
	})

	t.Run("admin caller that is not the last admin is deleted", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		roles := &mockUserRolesRepository{adminCount: 2}
		uc := NewUserUsecase(repo, roles, &mockAdminChecker{isAdmin: true}, newTestLogger())

		if err := uc.DeleteMyAccount(authedCtx(caller)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.deleteAuthCalls != 1 {
			t.Fatalf("DeleteAuthUser calls=%d, want 1", repo.deleteAuthCalls)
		}
	})

	t.Run("last admin is forbidden", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		roles := &mockUserRolesRepository{adminCount: 1}
		uc := NewUserUsecase(repo, roles, &mockAdminChecker{isAdmin: true}, newTestLogger())

		err := uc.DeleteMyAccount(authedCtx(caller))

		assertForbidden(t, err, "cannot delete the last admin account; promote another admin first")
		if repo.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUser must not run for the last admin, got %d calls", repo.deleteAuthCalls)
		}
	})

	t.Run("missing account is idempotent success", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{deleteAuthErr: repository.ErrNotFound}
		roles := &mockUserRolesRepository{}
		uc := NewUserUsecase(repo, roles, &mockAdminChecker{isAdmin: false}, newTestLogger())

		if err := uc.DeleteMyAccount(authedCtx(caller)); err != nil {
			t.Fatalf("expected nil (idempotent), got %v", err)
		}
	})

	t.Run("infrastructure error wraps as internal chain", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{deleteAuthErr: errors.New("boom")}
		roles := &mockUserRolesRepository{}
		uc := NewUserUsecase(repo, roles, &mockAdminChecker{isAdmin: false}, newTestLogger())

		assertInternalChain(t, uc.DeleteMyAccount(authedCtx(caller)), "usecase: user: delete my account")
	})

	t.Run("context cancellation propagates", func(t *testing.T) {
		t.Parallel()
		uc := NewUserUsecase(&mockUserRepository{}, &mockUserRolesRepository{}, &mockAdminChecker{err: context.Canceled}, newTestLogger())
		assertCancelled(t, uc.DeleteMyAccount(authedCtx(caller)))
	})

	t.Run("missing guard deps fail safe", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		uc := NewUserUsecase(repo, nil, &mockAdminChecker{}, newTestLogger())

		assertInternalChain(t, uc.DeleteMyAccount(authedCtx(caller)), "admin guard deps not configured")
		if repo.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUser must not run when guard deps are missing, got %d calls", repo.deleteAuthCalls)
		}
	})
}

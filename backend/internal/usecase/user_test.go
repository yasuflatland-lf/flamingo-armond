package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
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

	// authUserMissing makes AuthUserExists report the auth.users row as gone,
	// i.e. the account was deleted while its JWT was still valid. The zero value
	// keeps the row present, which is the handle_new_user provisioning race.
	authUserMissing bool
	authUserErr     error
}

func (m *mockUserRepository) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return m.findResult, m.findErr
}

func (m *mockUserRepository) Update(_ context.Context, _ string, patch repository.UserUpdate) (*domain.User, error) {
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

func (m *mockUserRepository) DeleteAuthUserTx(_ context.Context, _ *gorm.DB, id string) error {
	m.deleteAuthCalls++
	m.lastDeleteAuthID = id
	return m.deleteAuthErr
}

func (m *mockUserRepository) AuthUserExists(_ context.Context, _ string) (bool, error) {
	return !m.authUserMissing, m.authUserErr
}

type mockUserRolesRepository struct {
	roles  []*domain.Role
	err    error
	calls  int
	userID string

	adminCount     int64
	countAdminsErr error
	lockErr        error
	lockCalls      int
	countCalls     int
}

func (m *mockUserRolesRepository) ListByUser(_ context.Context, userID string) ([]*domain.Role, error) {
	m.calls++
	m.userID = userID
	return m.roles, m.err
}

// AcquireAdminRoleLockTx records that the guard serialized before counting; the
// counter asserts the lock is taken ahead of every count.
func (m *mockUserRolesRepository) AcquireAdminRoleLockTx(_ context.Context, _ *gorm.DB) error {
	m.lockCalls++
	return m.lockErr
}

func (m *mockUserRolesRepository) CountAdminsTx(_ context.Context, _ *gorm.DB) (int64, error) {
	m.countCalls++
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
		name            string
		ctx             context.Context
		findResult      *domain.User
		findErr         error
		authUserMissing bool
		authUserErr     error
		wantErr         string // expected outcome label: "UNAUTHENTICATED" (sentinel) | "INTERNAL" (eris-wrapped chain) | "" (no error)
		wantInternal    string // substring the INTERNAL chain must carry; defaults to the find-user-by-ID wrap
		wantID          string
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
			name:    "ErrNotFound with auth row present returns empty user with no error",
			ctx:     authedCtx("u1"),
			findErr: repository.ErrNotFound,
			wantID:  "u1",
		},
		{
			name:            "ErrNotFound with auth row gone returns UNAUTHENTICATED",
			ctx:             authedCtx("u1"),
			findErr:         repository.ErrNotFound,
			authUserMissing: true,
			wantErr:         "UNAUTHENTICATED",
		},
		{
			name:         "auth-row probe failure returns INTERNAL",
			ctx:          authedCtx("u1"),
			findErr:      repository.ErrNotFound,
			authUserErr:  errors.New("auth schema unreachable"),
			wantErr:      "INTERNAL",
			wantInternal: "usecase: user: me: auth user exists",
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
			repo := &mockUserRepository{
				findResult:      tc.findResult,
				findErr:         tc.findErr,
				authUserMissing: tc.authUserMissing,
				authUserErr:     tc.authUserErr,
			}
			uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

			p, err := uc.Me(tc.ctx)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				switch tc.wantErr {
				case "UNAUTHENTICATED":
					assertUnauthenticated(t, err)
				case "INTERNAL":
					wantSubstr := tc.wantInternal
					if wantSubstr == "" {
						wantSubstr = "usecase: user: me: find user by ID"
					}
					assertInternalChain(t, err, wantSubstr)
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
			if string(p.ID) != tc.wantID {
				t.Fatalf("expected user.ID=%q, got %q", tc.wantID, p.ID)
			}
		})
	}
}

// TestUserUsecase_Me_ProvisioningRace_LogsWarn pins the log side of the
// degrade branch: when the auth.users row is still present, the missing
// public.users row is the handle_new_user provisioning race, and the empty-user
// return must be announced at WARN so the race stays observable. The
// deleted-account branch returns before this log, so a warn line here also
// proves the two cases did not collapse.
func TestUserUsecase_Me_ProvisioningRace_LogsWarn(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	repo := &mockUserRepository{findErr: repository.ErrNotFound}
	uc := NewUserUsecase(nil, repo, nil, nil, logger)

	p, err := uc.Me(authedCtx("u1"))
	if err != nil {
		t.Fatalf("Me: unexpected error: %v", err)
	}
	if p == nil || string(p.ID) != "u1" {
		t.Fatalf("Me: expected empty user with ID=u1, got %#v", p)
	}

	out := buf.String()
	if !strings.Contains(out, `"level":"WARN"`) {
		t.Fatalf("expected a WARN log line, got %q", out)
	}
	if !strings.Contains(out, "user row missing for authenticated user") {
		t.Fatalf("expected the degrade warn message, got %q", out)
	}
}

// TestUserUsecase_Me_DeletedAccount_NoWarn proves the deleted-account branch
// does not reuse the provisioning-race warn: an operator seeing that line would
// go looking for a broken trigger.
func TestUserUsecase_Me_DeletedAccount_NoWarn(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	repo := &mockUserRepository{findErr: repository.ErrNotFound, authUserMissing: true}
	uc := NewUserUsecase(nil, repo, nil, nil, logger)

	if _, err := uc.Me(authedCtx("u1")); !errors.Is(err, ucerr.ErrUnauthenticated) {
		t.Fatalf("Me: expected ucerr.ErrUnauthenticated, got %v", err)
	}
	if out := buf.String(); strings.Contains(out, "user row missing for authenticated user") {
		t.Fatalf("deleted account must not emit the provisioning-race warn, got %q", out)
	}
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
			uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

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
					assertInternalChain(t, err, "usecase: user: update: update user")
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
	uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

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
	uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

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
	uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

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
	uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

	_, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{DisplayName: "Alice"})

	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	assertInternalChain(t, err, "usecase: user: update: update user")
}

func TestUserUsecase_UpdateUser_PropagatesCancelled(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepository{updateErr: context.Canceled}
	uc := NewUserUsecase(nil, repo, nil, nil, newTestLogger())

	outcome, err := uc.UpdateUser(authedCtx("u1"), UpdateUserInput{DisplayName: "Alice"})

	if outcome.User != nil || outcome.Validation != nil {
		t.Fatalf("expected empty outcome, got %+v", outcome)
	}
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %T: %v", err, err)
	}
}

// TestUserUsecase_DeleteMyAccount exercises the self-service account-deletion
// guard chain and the happy path.
func TestUserUsecase_DeleteMyAccount(t *testing.T) {
	t.Parallel()

	const caller = "user-1"

	t.Run("unauthenticated", func(t *testing.T) {
		t.Parallel()
		uc := NewUserUsecase(nil, &mockUserRepository{}, &mockUserRolesRepository{}, &mockAdminChecker{}, newTestLogger())
		assertUnauthenticated(t, uc.DeleteMyAccount(anonCtx()))
	})

	t.Run("non-admin caller is deleted without consulting the admin count", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		// adminCount is a blocking value to prove it is never consulted for non-admins.
		roles := &mockUserRolesRepository{adminCount: 1}
		lockCallsAtIsAdmin := -1
		authChk := &mockAdminChecker{isAdmin: false}
		authChk.onCall = func() { lockCallsAtIsAdmin = roles.lockCalls }
		uc := NewUserUsecase(nil, repo, roles, authChk, newTestLogger())

		if err := uc.DeleteMyAccount(authedCtx(caller)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.deleteAuthCalls != 1 || repo.lastDeleteAuthID != caller {
			t.Fatalf("DeleteAuthUserTx: calls=%d id=%q, want 1 and %q", repo.deleteAuthCalls, repo.lastDeleteAuthID, caller)
		}
		if roles.countCalls != 0 {
			t.Fatalf("count=%d, want 0 for a non-admin caller", roles.countCalls)
		}
		// The lock is still taken once: the membership read that decides the
		// caller is a non-admin must itself happen under it, otherwise a caller
		// promoted concurrently is read as a non-admin and skips the count.
		if roles.lockCalls != 1 || lockCallsAtIsAdmin != 1 {
			t.Fatalf("lock=%d lockCallsAtIsAdmin=%d, want the membership read taken once under the lock",
				roles.lockCalls, lockCallsAtIsAdmin)
		}
	})

	t.Run("admin caller that is not the last admin is deleted", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		roles := &mockUserRolesRepository{adminCount: 2}
		uc := NewUserUsecase(nil, repo, roles, &mockAdminChecker{isAdmin: true}, newTestLogger())

		if err := uc.DeleteMyAccount(authedCtx(caller)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.deleteAuthCalls != 1 {
			t.Fatalf("DeleteAuthUserTx calls=%d, want 1", repo.deleteAuthCalls)
		}
	})

	t.Run("last admin is forbidden", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		roles := &mockUserRolesRepository{adminCount: 1}
		uc := NewUserUsecase(nil, repo, roles, &mockAdminChecker{isAdmin: true}, newTestLogger())

		err := uc.DeleteMyAccount(authedCtx(caller))

		assertForbidden(t, err, "cannot delete the last admin account; promote another admin first")
		if repo.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUserTx must not run for the last admin, got %d calls", repo.deleteAuthCalls)
		}
		// The count is only trustworthy while the admin-role advisory lock is
		// held; without it a concurrent admin removal races past it.
		if roles.lockCalls != 1 || roles.countCalls != 1 {
			t.Fatalf("lock=%d count=%d, want the admin count read once under the lock",
				roles.lockCalls, roles.countCalls)
		}
	})

	t.Run("advisory lock failure fails closed", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		roles := &mockUserRolesRepository{adminCount: 2, lockErr: errors.New("boom")}
		authChk := &mockAdminChecker{isAdmin: true}
		uc := NewUserUsecase(nil, repo, roles, authChk, newTestLogger())

		err := uc.DeleteMyAccount(authedCtx(caller))

		// Swallowing the lock error and counting anyway would reopen the race
		// the lock closes, so the request must abort before the membership
		// read, before the count, and before the delete.
		assertInternalChain(t, err, "usecase: user: delete my account: count admins")
		if authChk.calls != 0 || roles.countCalls != 0 {
			t.Fatalf("isAdmin=%d count=%d, want neither once the lock could not be taken",
				authChk.calls, roles.countCalls)
		}
		if repo.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUserTx calls = %d, want 0 once the lock could not be taken", repo.deleteAuthCalls)
		}
	})

	t.Run("advisory lock cancellation propagates unwrapped", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		roles := &mockUserRolesRepository{lockErr: context.Canceled}
		uc := NewUserUsecase(nil, repo, roles, &mockAdminChecker{isAdmin: true}, newTestLogger())

		assertCancelled(t, uc.DeleteMyAccount(authedCtx(caller)))
		if repo.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUserTx calls = %d, want 0", repo.deleteAuthCalls)
		}
	})

	t.Run("missing account is idempotent success", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{deleteAuthErr: repository.ErrNotFound}
		roles := &mockUserRolesRepository{}
		uc := NewUserUsecase(nil, repo, roles, &mockAdminChecker{isAdmin: false}, newTestLogger())

		if err := uc.DeleteMyAccount(authedCtx(caller)); err != nil {
			t.Fatalf("expected nil (idempotent), got %v", err)
		}
	})

	t.Run("infrastructure error wraps as internal chain", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{deleteAuthErr: errors.New("boom")}
		roles := &mockUserRolesRepository{}
		uc := NewUserUsecase(nil, repo, roles, &mockAdminChecker{isAdmin: false}, newTestLogger())

		assertInternalChain(t, uc.DeleteMyAccount(authedCtx(caller)), "usecase: user: delete my account")
	})

	t.Run("context cancellation propagates", func(t *testing.T) {
		t.Parallel()
		uc := NewUserUsecase(nil, &mockUserRepository{}, &mockUserRolesRepository{}, &mockAdminChecker{err: context.Canceled}, newTestLogger())
		assertCancelled(t, uc.DeleteMyAccount(authedCtx(caller)))
	})

	t.Run("missing guard deps fail safe", func(t *testing.T) {
		t.Parallel()
		repo := &mockUserRepository{}
		uc := NewUserUsecase(nil, repo, nil, &mockAdminChecker{}, newTestLogger())

		assertInternalChain(t, uc.DeleteMyAccount(authedCtx(caller)), "admin guard deps not configured")
		if repo.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUserTx must not run when guard deps are missing, got %d calls", repo.deleteAuthCalls)
		}
	})
}

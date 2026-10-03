package loader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/loader"
	"backend/internal/repository"
)

// countingRepo is a function-table test double for repository.UserRepository.
// Unconfigured methods panic so an unexpected call fails loudly.
type countingRepo struct {
	findByID            func(ctx context.Context, id string) (*domain.User, error)
	update              func(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error)
	lastSignInByUserIDs func(ctx context.Context, ids []string) (map[string]*time.Time, error)
}

// emptyUserRoleRepoStub is a UserRoleRepository stub where only ListByUserIDs
// is configured (returns empty map). All other methods panic if called.
type emptyUserRoleRepoStub struct{}

func (emptyUserRoleRepoStub) HasRole(_ context.Context, _ string, _ domain.RoleName) (bool, error) {
	panic("emptyUserRoleRepoStub.HasRole not expected")
}
func (emptyUserRoleRepoStub) HasRoleTx(_ context.Context, _ *gorm.DB, _ string, _ domain.RoleName) (bool, error) {
	panic("emptyUserRoleRepoStub.HasRoleTx not expected")
}
func (emptyUserRoleRepoStub) AssignRoleToUser(_ context.Context, _, _ string) error {
	panic("emptyUserRoleRepoStub.AssignRoleToUser not expected")
}
func (emptyUserRoleRepoStub) SetUserRolesTx(_ context.Context, _ *gorm.DB, _ string, _ []string) error {
	panic("emptyUserRoleRepoStub.SetUserRolesTx not expected")
}
func (emptyUserRoleRepoStub) ListByUser(_ context.Context, _ string) ([]*domain.Role, error) {
	panic("emptyUserRoleRepoStub.ListByUser not expected")
}
func (emptyUserRoleRepoStub) ListByUserIDs(_ context.Context, _ []string) (map[string][]*domain.Role, error) {
	return map[string][]*domain.Role{}, nil
}
func (emptyUserRoleRepoStub) CountAdmins(_ context.Context) (int64, error) {
	panic("emptyUserRoleRepoStub.CountAdmins not expected")
}
func (emptyUserRoleRepoStub) CountAdminsTx(_ context.Context, _ *gorm.DB) (int64, error) {
	panic("emptyUserRoleRepoStub.CountAdminsTx not expected")
}
func (emptyUserRoleRepoStub) AcquireAdminRoleLockTx(_ context.Context, _ *gorm.DB) error {
	panic("emptyUserRoleRepoStub.AcquireAdminRoleLockTx not expected")
}

func emptyUserRoleRepo() repository.UserRoleRepository {
	return emptyUserRoleRepoStub{}
}

// countingCardgroupRepo is a minimal test double for repository.CardgroupRepository.
type countingCardgroupRepo struct {
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error)
}

type countingCardRepo struct {
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.Card, error)
}

func (r *countingCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	panic("countingCardgroupRepo.FindByID not configured")
}
func (r *countingCardgroupRepo) FindByName(_ context.Context, _, _ string) (*domain.Cardgroup, error) {
	panic("countingCardgroupRepo.FindByName not configured")
}
func (r *countingCardgroupRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error) {
	if r.findByIDs == nil {
		panic("countingCardgroupRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}
func (r *countingCardgroupRepo) Create(_ context.Context, _ *domain.Cardgroup) error {
	panic("countingCardgroupRepo.Create not configured")
}
func (r *countingCardgroupRepo) CreateTx(_ context.Context, _ *gorm.DB, _ *domain.Cardgroup) error {
	panic("countingCardgroupRepo.CreateTx not configured")
}
func (r *countingCardgroupRepo) CountByOwnerTx(_ context.Context, _ *gorm.DB, _ string) (int64, error) {
	panic("countingCardgroupRepo.CountByOwnerTx not configured")
}
func (r *countingCardgroupRepo) AcquireUserCardgroupLockTx(_ context.Context, _ *gorm.DB, _ string) error {
	panic("countingCardgroupRepo.AcquireUserCardgroupLockTx not configured")
}
func (r *countingCardgroupRepo) Update(_ context.Context, _ string, _ repository.CardgroupUpdate) (*domain.Cardgroup, error) {
	panic("countingCardgroupRepo.Update not configured")
}
func (r *countingCardgroupRepo) Delete(_ context.Context, _ string) error {
	panic("countingCardgroupRepo.Delete not configured")
}
func (r *countingCardgroupRepo) FindPageByOwner(
	_ context.Context,
	_ string,
	_, _ *repository.CardgroupCursor,
	_, _ int,
	_ repository.CardgroupOrderBy,
	_ repository.SortOrder,
	_ *string,
) ([]*domain.Cardgroup, int64, error) {
	panic("countingCardgroupRepo.FindPageByOwner not configured")
}
func (r *countingCardgroupRepo) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	panic("countingCardgroupRepo.CountByOwner not configured")
}

func emptyCardgroupRepo() *countingCardgroupRepo {
	return &countingCardgroupRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Cardgroup, error) {
			return map[string]*domain.Cardgroup{}, nil
		},
	}
}

func (r *countingCardRepo) FindByID(_ context.Context, _ string) (*domain.Card, error) {
	panic("countingCardRepo.FindByID not configured")
}
func (r *countingCardRepo) FindByIDForUpdateTx(_ context.Context, _ *gorm.DB, _ string) (*domain.Card, error) {
	panic("countingCardRepo.FindByIDForUpdateTx not configured")
}
func (r *countingCardRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error) {
	if r.findByIDs == nil {
		panic("countingCardRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}
func (r *countingCardRepo) ListByCardgroup(_ context.Context, _ string) ([]*domain.Card, error) {
	panic("countingCardRepo.ListByCardgroup not configured")
}
func (r *countingCardRepo) ListFrontsByCardgroupTx(_ context.Context, _ *gorm.DB, _ string) ([]string, error) {
	panic("countingCardRepo.ListFrontsByCardgroupTx not configured")
}
func (r *countingCardRepo) FindPageByCardgroup(
	_ context.Context, _ string,
	_, _ *repository.CardCursor,
	_, _ int,
	_ repository.CardOrderBy, _ repository.SortOrder,
	_ *string,
) ([]*domain.Card, int64, error) {
	panic("countingCardRepo.FindPageByCardgroup not configured")
}
func (r *countingCardRepo) FindPageByCardgroupForUser(
	_ context.Context, _, _ string,
	_, _ *repository.CardCursor,
	_, _ int,
	_ repository.CardOrderBy, _ repository.SortOrder,
	_ *string,
) ([]*domain.Card, int64, map[string]time.Time, error) {
	panic("countingCardRepo.FindPageByCardgroupForUser not configured")
}
func (r *countingCardRepo) Create(_ context.Context, _ *domain.Card) error {
	panic("countingCardRepo.Create not configured")
}
func (r *countingCardRepo) Update(_ context.Context, _ string, _ repository.CardUpdate) (*domain.Card, error) {
	panic("countingCardRepo.Update not configured")
}
func (r *countingCardRepo) Delete(_ context.Context, _ string) error {
	panic("countingCardRepo.Delete not configured")
}
func (r *countingCardRepo) DeleteByIDsTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (int64, error) {
	panic("countingCardRepo.DeleteByIDsTx not configured")
}
func (r *countingCardRepo) DeleteByCardgroupAndFrontsTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (int64, error) {
	panic("countingCardRepo.DeleteByCardgroupAndFrontsTx not configured")
}
func (r *countingCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, _ []*domain.Card) (repository.UpsertManyTxResult, error) {
	panic("countingCardRepo.UpsertManyTx not configured")
}
func (r *countingCardRepo) FoldFrontCaseToTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (int64, error) {
	panic("countingCardRepo.FoldFrontCaseToTx not configured")
}
func (r *countingCardRepo) FindByCardgroupAndFront(_ context.Context, _, _ string) (*domain.Card, error) {
	panic("countingCardRepo.FindByCardgroupAndFront not configured")
}
func (r *countingCardRepo) CountMatchingFrontsFold(_ context.Context, _ string, _ []string) (int64, error) {
	panic("countingCardRepo.CountMatchingFrontsFold not configured")
}

func emptyCardRepo() *countingCardRepo {
	return &countingCardRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Card, error) {
			return map[string]*domain.Card{}, nil
		},
	}
}

func (r *countingRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if r.findByID == nil {
		panic("countingRepo.FindByID not configured")
	}
	return r.findByID(ctx, id)
}

func (r *countingRepo) Update(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error) {
	if r.update == nil {
		panic("countingRepo.Update not configured")
	}
	return r.update(ctx, id, patch)
}

func (r *countingRepo) UpdateTx(_ context.Context, _ *gorm.DB, _ string, _ repository.UserUpdate) error {
	panic("countingRepo.UpdateTx not configured")
}

func (r *countingRepo) UpdateTxVersioned(_ context.Context, _ *gorm.DB, _ string, _ repository.UserUpdate, _ int64) error {
	panic("countingRepo.UpdateTxVersioned not configured")
}

// ListPage satisfies repository.UserRepository. The loader-layer tests never
// hit cursor pagination, so this fixture panics if called — surfacing any
// accidental coupling instead of silently returning a fabricated empty page.
func (r *countingRepo) ListPage(
	_ context.Context,
	_, _ *string,
	_, _ int,
	_ *string,
) ([]*domain.User, int64, error) {
	panic("countingRepo.ListPage not configured")
}

// DeleteAuthUserTx satisfies repository.UserRepository. Loader-layer tests never
// invoke account deletion; panic if called so accidental coupling is surfaced.
func (r *countingRepo) DeleteAuthUserTx(_ context.Context, _ *gorm.DB, _ string) error {
	panic("countingRepo.DeleteAuthUserTx not configured")
}

// AuthUserExists satisfies repository.UserRepository. Loader-layer tests never
// probe auth-row existence; panic if called so accidental coupling is surfaced.
func (r *countingRepo) AuthUserExists(_ context.Context, _ string) (bool, error) {
	panic("countingRepo.AuthUserExists not configured")
}

// LastSignInByUserIDs satisfies repository.UserRepository. Configure the
// lastSignInByUserIDs func to exercise the LastSignInByUserID loader; the
// card/cardgroup loader tests leave it nil and panic if it is unexpectedly
// invoked.
func (r *countingRepo) LastSignInByUserIDs(ctx context.Context, ids []string) (map[string]*time.Time, error) {
	if r.lastSignInByUserIDs == nil {
		panic("countingRepo.LastSignInByUserIDs not configured")
	}
	return r.lastSignInByUserIDs(ctx, ids)
}

// SetLastViewedCardgroup satisfies repository.UserRepository. Loader-layer
// tests never invoke this path; panic if called so accidental coupling is
// surfaced rather than silently no-oped.
func (r *countingRepo) SetLastViewedCardgroup(_ context.Context, _, _ string) error {
	panic("countingRepo.SetLastViewedCardgroup not configured")
}

// countingUserPreferenceRepo is a minimal test double for
// userPreferenceReader. Tests that do not exercise the UserPreference loader
// use emptyUserPreferenceRepo() so the stub is always a no-op.
type countingUserPreferenceRepo struct {
	findByUserIDs func(ctx context.Context, userIDs []string) ([]*domain.UserPreference, error)
}

func (r *countingUserPreferenceRepo) FindByUserIDs(ctx context.Context, userIDs []string) ([]*domain.UserPreference, error) {
	if r.findByUserIDs == nil {
		panic("countingUserPreferenceRepo.FindByUserIDs not configured")
	}
	return r.findByUserIDs(ctx, userIDs)
}

// FindByUserID is needed to satisfy repository.UserPreferenceRepository.
func (r *countingUserPreferenceRepo) FindByUserID(_ context.Context, _ string) (*domain.UserPreference, error) {
	panic("countingUserPreferenceRepo.FindByUserID not configured")
}

// UpsertLastViewedCardgroup is needed to satisfy repository.UserPreferenceRepository.
func (r *countingUserPreferenceRepo) UpsertLastViewedCardgroup(_ context.Context, _, _ string) error {
	panic("countingUserPreferenceRepo.UpsertLastViewedCardgroup not configured")
}

// UpsertLearnDisplayMode is needed to satisfy repository.UserPreferenceRepository.
func (r *countingUserPreferenceRepo) UpsertLearnDisplayMode(_ context.Context, _, _ string) error {
	panic("countingUserPreferenceRepo.UpsertLearnDisplayMode not configured")
}

// UpsertNewCardRatio is needed to satisfy repository.UserPreferenceRepository.
func (r *countingUserPreferenceRepo) UpsertNewCardRatio(_ context.Context, _ string, _, _ int) error {
	panic("countingUserPreferenceRepo.UpsertNewCardRatio not configured")
}

func emptyUserPreferenceRepo() *countingUserPreferenceRepo {
	return &countingUserPreferenceRepo{
		findByUserIDs: func(_ context.Context, _ []string) ([]*domain.UserPreference, error) {
			return []*domain.UserPreference{}, nil
		},
	}
}

func TestMiddleware_For_Roundtrip(t *testing.T) {
	t.Parallel()

	repo := &countingRepo{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := e.NewContext(req, httptest.NewRecorder())

	var got *loader.Loaders
	handler := func(c *echo.Context) error {
		got = loader.For(c.Request().Context())
		return nil
	}
	if err := loader.Middleware(repo, emptyUserRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), emptyUserPreferenceRepo(), nil)(handler)(c); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if got == nil {
		t.Fatalf("loader.For returned nil; middleware did not install Loaders")
	}
	if got.Cardgroup == nil {
		t.Fatalf("Loaders.Cardgroup is nil")
	}
	if got.UserCardFSRS != nil {
		t.Fatalf("Loaders.UserCardFSRS is not nil without a viewer or reader")
	}
}

func TestMiddleware_InstallsViewerScopedUserCardFSRS(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		viewer string
	}{
		{name: "authenticated", viewer: "viewer-1"},
		{name: "anonymous"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedUserID atomic.Value
			repo := &countingUserCardFSRSRepo{
				findByUserAndCardIDs: func(_ context.Context, userID string, _ []string) (map[string]*domain.UserCardFSRS, error) {
					receivedUserID.Store(userID)
					return map[string]*domain.UserCardFSRS{}, nil
				},
			}
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.viewer != "" {
				req = req.WithContext(auth.ContextWithUser(req.Context(), &auth.AuthUser{Sub: tt.viewer}))
			}
			c := e.NewContext(req, httptest.NewRecorder())

			var got *loader.Loaders
			handler := func(c *echo.Context) error {
				got = loader.For(c.Request().Context())
				return nil
			}
			if err := loader.Middleware(emptyUserRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), emptyUserPreferenceRepo(), repo)(handler)(c); err != nil {
				t.Fatalf("middleware: %v", err)
			}
			if got == nil {
				t.Fatalf("loader.For returned nil; middleware did not install Loaders")
			}
			if tt.viewer == "" {
				if got.UserCardFSRS != nil {
					t.Fatalf("Loaders.UserCardFSRS is not nil without a viewer")
				}
				return
			}
			if got.UserCardFSRS == nil {
				t.Fatalf("Loaders.UserCardFSRS is nil for an authenticated viewer")
			}
			if _, err := got.UserCardFSRS.Load(context.Background(), "card-1")(); err != nil {
				t.Fatalf("UserCardFSRS.Load: %v", err)
			}
			if userID := receivedUserID.Load(); userID != tt.viewer {
				t.Fatalf("FindByUserAndCardIDs userID = %v, want %q", userID, tt.viewer)
			}
		})
	}
}

func TestFor_NoLoaders_ReturnsNil(t *testing.T) {
	t.Parallel()
	if l := loader.For(context.Background()); l != nil {
		t.Fatalf("expected nil from bare context, got %+v", l)
	}
}

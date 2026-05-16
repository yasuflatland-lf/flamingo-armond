package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/labstack/echo/v5"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// userCardFSRSReader is the narrow interface the loader needs from the
// UserCardFSRS repository. The loader only calls FindByUserAndCardIDs; it
// never calls UpsertTx, so only that method is listed here.
type userCardFSRSReader interface {
	FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
}

type contextKey struct{}

type Loaders struct {
	User           *dataloader.Loader[string, *domain.User]
	Role           *dataloader.Loader[string, *domain.Role]
	RoleByUserID   *RoleByUserIDLoader
	Cardgroup      *dataloader.Loader[string, *domain.Cardgroup]
	Card           *dataloader.Loader[string, *domain.Card]
	SwipeRecord    *dataloader.Loader[string, *domain.SwipeRecord]
	UserCardFSRS   *dataloader.Loader[string, *domain.UserCardFSRS]
	UserPreference *UserPreferenceLoader
}

func New(userRepo repository.UserRepository, roleRepo repository.RoleRepository, cardgroupRepo repository.CardgroupRepository, cardRepo repository.CardRepository, userPreferenceRepo repository.UserPreferenceRepository, swipeRecordRepo ...repository.SwipeRecordRepository) *Loaders {
	loaders := &Loaders{
		User:           dataloader.NewBatchedLoader(userBatchFunc(userRepo)),
		Role:           dataloader.NewBatchedLoader(roleBatchFunc(roleRepo)),
		RoleByUserID:   dataloader.NewBatchedLoader(roleByUserIDBatchFunc(roleRepo)),
		Cardgroup:      dataloader.NewBatchedLoader(cardgroupBatchFunc(cardgroupRepo)),
		Card:           dataloader.NewBatchedLoader(cardBatchFunc(cardRepo)),
		UserPreference: NewUserPreferenceLoader(userPreferenceRepo),
	}
	if len(swipeRecordRepo) > 0 && swipeRecordRepo[0] != nil {
		loaders.SwipeRecord = dataloader.NewBatchedLoader(swipeRecordBatchFunc(swipeRecordRepo[0]))
	}
	return loaders
}

func NewWithUserCardFSRS(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	cardgroupRepo repository.CardgroupRepository,
	cardRepo repository.CardRepository,
	userPreferenceRepo repository.UserPreferenceRepository,
	swipeRecordRepo repository.SwipeRecordRepository,
	userCardFSRSRepo userCardFSRSReader,
	viewer string,
) *Loaders {
	// New tolerates a nil SwipeRecordRepository inside the variadic slot, so
	// forward unconditionally; the nil-check lives there.
	loaders := New(userRepo, roleRepo, cardgroupRepo, cardRepo, userPreferenceRepo, swipeRecordRepo)
	if userCardFSRSRepo != nil && viewer != "" {
		loaders.UserCardFSRS = dataloader.NewBatchedLoader(userCardFSRSBatchFunc(userCardFSRSRepo, viewer))
	}
	return loaders
}

// Middleware installs a fresh Loaders per request so batching and caching do
// not bleed across requests.
func Middleware(userRepo repository.UserRepository, roleRepo repository.RoleRepository, cardgroupRepo repository.CardgroupRepository, cardRepo repository.CardRepository, userPreferenceRepo repository.UserPreferenceRepository, swipeRecordRepo ...repository.SwipeRecordRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), contextKey{}, New(userRepo, roleRepo, cardgroupRepo, cardRepo, userPreferenceRepo, swipeRecordRepo...))
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func MiddlewareWithUserCardFSRS(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	cardgroupRepo repository.CardgroupRepository,
	cardRepo repository.CardRepository,
	userPreferenceRepo repository.UserPreferenceRepository,
	swipeRecordRepo repository.SwipeRecordRepository,
	userCardFSRSRepo userCardFSRSReader,
) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			viewer := ""
			if user := auth.UserFrom(c.Request().Context()); user != nil {
				viewer = user.Sub
			}
			ctx := context.WithValue(c.Request().Context(), contextKey{}, NewWithUserCardFSRS(
				userRepo, roleRepo, cardgroupRepo, cardRepo, userPreferenceRepo, swipeRecordRepo, userCardFSRSRepo, viewer,
			))
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// For returns the per-request Loaders set by Middleware, or nil if Middleware
// did not run.
func For(ctx context.Context) *Loaders {
	l, _ := ctx.Value(contextKey{}).(*Loaders)
	return l
}

// WithContext stores loaders in ctx and returns the enriched context. Use
// this in tests to inject a Loaders without going through the Echo middleware.
func WithContext(ctx context.Context, l *Loaders) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

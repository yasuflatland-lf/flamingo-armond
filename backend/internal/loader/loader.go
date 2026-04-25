package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/labstack/echo/v5"

	"backend/internal/domain"
	"backend/internal/repository"
)

type contextKey struct{}

type Loaders struct {
	Profile *dataloader.Loader[string, *domain.Profile]
}

func New(repo repository.ProfileRepository) *Loaders {
	return &Loaders{
		Profile: dataloader.NewBatchedLoader(profileBatchFunc(repo)),
	}
}

// Middleware installs a fresh Loaders per request so batching and caching do
// not bleed across requests.
func Middleware(repo repository.ProfileRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), contextKey{}, New(repo))
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

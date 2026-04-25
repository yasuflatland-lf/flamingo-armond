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

// Middleware installs a per-request Loaders into the request context so each
// HTTP request gets a fresh batch/cache and loads do not bleed across requests.
func Middleware(repo repository.ProfileRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ldr := New(repo)
			ctx := context.WithValue(c.Request().Context(), contextKey{}, ldr)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// For returns the per-request Loaders set by Middleware, or nil if Middleware
// did not run on this request (in which case calling .Load on a loader will panic).
func For(ctx context.Context) *Loaders {
	l, _ := ctx.Value(contextKey{}).(*Loaders)
	return l
}

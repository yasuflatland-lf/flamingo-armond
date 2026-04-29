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
	User *dataloader.Loader[string, *domain.User]
	Role *dataloader.Loader[string, *domain.Role]
}

func New(userRepo repository.UserRepository, roleRepo repository.RoleRepository) *Loaders {
	return &Loaders{
		User: dataloader.NewBatchedLoader(userBatchFunc(userRepo)),
		Role: dataloader.NewBatchedLoader(roleBatchFunc(roleRepo)),
	}
}

// Middleware installs a fresh Loaders per request so batching and caching do
// not bleed across requests.
func Middleware(userRepo repository.UserRepository, roleRepo repository.RoleRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), contextKey{}, New(userRepo, roleRepo))
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

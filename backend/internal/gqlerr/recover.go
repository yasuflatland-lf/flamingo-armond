package gqlerr

import (
	"context"
	"runtime/debug"

	"github.com/rotisserie/eris"
)

// RecoverFunc satisfies graphql.RecoverFunc. It converts a recovered panic
// value into a gqlerr.Internal error, preserving the goroutine stack at the
// recovery point.
func RecoverFunc(ctx context.Context, err any) error {
	stack := debug.Stack()
	return Internal(ctx,
		eris.Errorf("graphql: panic recovered (%T %v)\n%s", err, err, stack),
	)
}

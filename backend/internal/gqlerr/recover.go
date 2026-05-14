package gqlerr

import (
	"context"
	"runtime/debug"

	"github.com/rotisserie/eris"
)

// RecoverFunc satisfies graphql.RecoverFunc. It converts a recovered panic
// value into an INTERNAL GraphQL error (extensions.code = "INTERNAL"), logs
// the full error chain and goroutine stack at ERROR level via gqlerr.Internal,
// and returns a sanitised "internal server error" message to the client.
func RecoverFunc(ctx context.Context, err any) error {
	stack := debug.Stack()
	return Internal(ctx,
		eris.Errorf("graphql: panic recovered (%T %v)\n%s", err, err, stack),
	)
}

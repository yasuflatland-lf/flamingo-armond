package resolver

import (
	"context"
	"strings"
	"testing"

	"github.com/rotisserie/eris"

	"backend/graph/model"
)

// TestCardImportKindOrPanic_InvalidRawPanics pins the resolver guard: a kind outside
// the generated enum panics with a recognizable chain, and a valid kind passes through.
func TestCardImportKindOrPanic_InvalidRawPanics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"not a kind", "NOT_A_KIND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("expected a panic")
				}
				err, ok := recovered.(error)
				if !ok {
					t.Fatalf("panic = %T, want error", recovered)
				}
				if chain := eris.ToString(err, true); !strings.Contains(chain, "invalid Kind escaped to resolver") {
					t.Fatalf("panic chain = %q, want invalid Kind escaped to resolver", chain)
				}
			}()
			cardImportKindOrPanic(context.Background(), tc.raw)
		})
	}

	t.Run("valid kind passes through", func(t *testing.T) {
		t.Parallel()
		if got := cardImportKindOrPanic(context.Background(), "DUPLICATE"); got != model.CardImportErrorKindDuplicate {
			t.Fatalf("cardImportKindOrPanic(DUPLICATE) = %q, want %q", got, model.CardImportErrorKindDuplicate)
		}
	})
}

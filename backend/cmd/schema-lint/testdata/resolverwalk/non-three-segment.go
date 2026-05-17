package resolver

import "context"

type fakeResolver struct{}

type mutationResolver struct{ *fakeResolver }

// FakeMutation contains call chains that do NOT match the three-segment
// <recv>.<Selector>.<Method>() pattern the walker looks for:
//
//   - r.Logger(): two-segment (method directly on recv), no third segment.
//   - r.A.B.C(): four-segment chain; inner.X is a SelectorExpr, not an Ident,
//     so the recvIdent check rejects it.
//   - localVar.Field.Method(): three-segment but recvIdent.Name != "r".
//
// None should appear in UsecaseCalls.
func (r *mutationResolver) FakeMutation(ctx context.Context) error {
	// Two-segment: direct method on receiver — the outer SelectorExpr's X is
	// already the recv Ident, so there is no inner SelectorExpr.
	_ = r.Enabled()

	// Four-segment: r.Sub.Child.DeepCall() — inner.X is SelectorExpr{X:r, Sel:Sub},
	// not a plain Ident, so the recvIdent assertion fails.
	r.Sub.Child.DeepCall()

	// Three-segment but the base is a local variable, not the receiver.
	// The recvIdent.Name check rejects it because "localVar" != "r".
	localVar := &struct{ Field interface{ Act() } }{}
	localVar.Field.Act()

	return nil
}

package resolver

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/graph/model"
	"backend/internal/usecase"
)

func cardImportKindOrPanic(ctx context.Context, raw string) model.CardImportErrorKind {
	k := model.CardImportErrorKind(raw)
	if !k.IsValid() {
		slog.ErrorContext(ctx, "card import: invalid Kind escaped to resolver - programmer bug", "raw", raw)
		panic(eris.Errorf("card import: invalid Kind escaped to resolver: %q", raw))
	}
	return k
}

func toCardImportValidationResultModel(ctx context.Context, out usecase.ValidateCardImportOutcome) *model.CardImportValidationResult {
	parsed := make([]*model.ParsedCard, 0, len(out.ParsedCards))
	for _, w := range out.ParsedCards {
		parsed = append(parsed, &model.ParsedCard{Front: w.Front, Back: w.Back, Line: w.Line})
	}
	return &model.CardImportValidationResult{
		Valid:       out.Valid,
		ParsedCards: parsed,
		Errors:      toCardImportErrors(ctx, out.Errors),
	}
}

func toCardImportErrors(ctx context.Context, errs []usecase.CardImportError) []*model.CardImportError {
	out := make([]*model.CardImportError, 0, len(errs))
	for _, e := range errs {
		out = append(out, &model.CardImportError{
			Line:    e.Line,
			Message: e.Message,
			Kind:    cardImportKindOrPanic(ctx, string(e.Kind)),
		})
	}
	return out
}

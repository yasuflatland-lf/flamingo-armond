package resolver

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/graph/model"
	"backend/internal/usecase"
)

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func dictionaryKindOrPanic(ctx context.Context, raw string) model.DictionaryValidationKind {
	if raw == "" || raw == string(model.DictionaryValidationKindUnknown) {
		slog.ErrorContext(ctx, "dictionary: UNKNOWN/empty Kind escaped to resolver - programmer bug",
			"raw", raw,
		)
		panic(eris.Errorf("dictionary: UNKNOWN/empty Kind escaped to resolver: %q", raw))
	}
	return model.DictionaryValidationKind(raw)
}

func toDictionaryValidationErrorsFromUpsert(ctx context.Context, errs []usecase.DictionaryValidationError) []*model.DictionaryValidationError {
	return toDictionaryValidationErrors(ctx, errs)
}

func toDictionaryValidationResultModel(ctx context.Context, out usecase.ValidateDictionaryOutcome) *model.DictionaryValidationResult {
	parsed := make([]*model.ParsedWord, 0, len(out.ParsedWords))
	for _, w := range out.ParsedWords {
		parsed = append(parsed, &model.ParsedWord{Front: w.Front, Back: w.Back, Line: w.Line})
	}
	return &model.DictionaryValidationResult{
		Valid:       out.Valid,
		ParsedWords: parsed,
		Errors:      toDictionaryValidationErrors(ctx, out.Errors),
	}
}

func toDictionaryValidationErrors(ctx context.Context, errs []usecase.DictionaryValidationError) []*model.DictionaryValidationError {
	out := make([]*model.DictionaryValidationError, 0, len(errs))
	for _, e := range errs {
		out = append(out, &model.DictionaryValidationError{
			Line:    e.Line,
			Message: e.Message,
			Kind:    dictionaryKindOrPanic(ctx, string(e.Kind)),
			Snippet: nilIfEmpty(e.Snippet),
			Front:   nilIfEmpty(e.Front),
			Back:    nilIfEmpty(e.Back),
		})
	}
	return out
}

package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

// UserPreferenceLoader is a DataLoader keyed by user_id that batches calls to
// UserPreferenceRepository.FindByUserIDs.
type UserPreferenceLoader = dataloader.Loader[string, *domain.UserPreference]

// userPreferenceReader is the narrow interface the loader needs from the
// UserPreference repository. Only FindByUserIDs is required.
type userPreferenceReader interface {
	FindByUserIDs(ctx context.Context, userIDs []string) ([]*domain.UserPreference, error)
}

// NewUserPreferenceLoader constructs a batched UserPreferenceLoader backed by
// the given repository.
func NewUserPreferenceLoader(repo userPreferenceReader) *UserPreferenceLoader {
	return dataloader.NewBatchedLoader(userPreferenceBatchFunc(repo))
}

func userPreferenceBatchFunc(repo userPreferenceReader) dataloader.BatchFunc[string, *domain.UserPreference] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.UserPreference] {
		out := make([]*dataloader.Result[*domain.UserPreference], len(keys))
		// Short-circuit the empty-slice case to avoid the GORM WHERE IN ()
		// full-table scan bug documented in go-library-gotchas.md.
		if len(keys) == 0 {
			return out
		}

		prefs, err := repo.FindByUserIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.UserPreference]{Error: err}
			}
			return out
		}

		byUserID := make(map[string]*domain.UserPreference, len(prefs))
		for _, p := range prefs {
			byUserID[string(p.UserID)] = p
		}

		// Missing keys yield nil data with nil error: absence means "no preference
		// set yet", not a fetch failure. Callers branch on pref == nil.
		for i, k := range keys {
			out[i] = &dataloader.Result[*domain.UserPreference]{Data: byUserID[k]}
		}
		return out
	}
}

// Ensure UserPreferenceRepository satisfies userPreferenceReader at compile time.
var _ userPreferenceReader = (repository.UserPreferenceRepository)(nil)

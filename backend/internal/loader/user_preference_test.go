package loader_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"backend/internal/domain"
	"backend/internal/loader"
)

// loadAllUserPreferences concurrently loads all userIDs and returns aligned results/errors.
func loadAllUserPreferences(ctx context.Context, l *loader.Loaders, userIDs []string) ([]*domain.UserPreference, []error) {
	results := make([]*domain.UserPreference, len(userIDs))
	errs := make([]error, len(userIDs))

	var wg sync.WaitGroup
	for i, id := range userIDs {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.UserPreference.Load(ctx, id)()
		}(i, id)
	}
	wg.Wait()
	return results, errs
}

// TestUserPreferenceLoader_BatchesNCallsIntoOne asserts that N concurrent
// Load calls collapse into a single FindByUserIDs invocation.
func TestUserPreferenceLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	var receivedIDs []string

	prefRepo := &countingUserPreferenceRepo{
		findByUserIDs: func(_ context.Context, userIDs []string) ([]*domain.UserPreference, error) {
			batchCalls.Add(1)
			receivedIDs = userIDs
			out := make([]*domain.UserPreference, len(userIDs))
			for i, id := range userIDs {
				out[i] = &domain.UserPreference{UserID: domain.UserID(id)}
			}
			return out, nil
		},
	}

	ids := []string{"u1", "u2", "u3", "u4", "u5"}
	l := loader.New(emptyUserRepo(), emptyRoleRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), prefRepo)
	results, errs := loadAllUserPreferences(context.Background(), l, ids)

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: unexpected error: %v", i, err)
		}
		if results[i] == nil {
			t.Fatalf("load %d: nil result", i)
		}
		if string(results[i].UserID) != ids[i] {
			t.Fatalf("load %d: UserID mismatch: got %q want %q", i, results[i].UserID, ids[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("FindByUserIDs should be called exactly once, called %d times", got)
	}
	if got := len(receivedIDs); got != len(ids) {
		t.Fatalf("batch should receive %d keys, got %d", len(ids), got)
	}
}

// TestUserPreferenceLoader_EmptyKeySliceShortCircuits verifies that the batch
// func stays cold when no Load calls are issued (exercises the empty-key guard).
func TestUserPreferenceLoader_EmptyKeySliceShortCircuits(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	prefRepo := &countingUserPreferenceRepo{
		findByUserIDs: func(_ context.Context, _ []string) ([]*domain.UserPreference, error) {
			calls.Add(1)
			return nil, errors.New("FindByUserIDs must not be called")
		},
	}

	_ = loader.New(emptyUserRepo(), emptyRoleRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), prefRepo)

	if got := calls.Load(); got != 0 {
		t.Fatalf("FindByUserIDs called %d times before any Load; want 0", got)
	}
}

// TestUserPreferenceLoader_MissingUserReturnsNilData asserts that an absent
// user_id produces a nil result (not an error) so resolvers treat it as defaults.
func TestUserPreferenceLoader_MissingUserReturnsNilData(t *testing.T) {
	t.Parallel()

	prefRepo := &countingUserPreferenceRepo{
		findByUserIDs: func(_ context.Context, userIDs []string) ([]*domain.UserPreference, error) {
			out := []*domain.UserPreference{}
			for _, id := range userIDs {
				if id == "present" {
					out = append(out, &domain.UserPreference{UserID: domain.UserID(id)})
				}
			}
			return out, nil
		},
	}

	l := loader.New(emptyUserRepo(), emptyRoleRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), prefRepo)
	results, errs := loadAllUserPreferences(context.Background(), l, []string{"present", "missing"})

	if errs[0] != nil {
		t.Fatalf("present: unexpected error: %v", errs[0])
	}
	if results[0] == nil || results[0].UserID != "present" {
		t.Fatalf("present: bad result: %+v", results[0])
	}
	if errs[1] != nil {
		t.Fatalf("missing: want nil error (absent row is not an error), got %v", errs[1])
	}
	if results[1] != nil {
		t.Fatalf("missing: want nil result, got %+v", results[1])
	}
}

// TestUserPreferenceLoader_BatchFuncError propagates a repository error to all
// keys in the batch.
func TestUserPreferenceLoader_BatchFuncError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("preference store exploded")
	prefRepo := &countingUserPreferenceRepo{
		findByUserIDs: func(_ context.Context, _ []string) ([]*domain.UserPreference, error) {
			return nil, wantErr
		},
	}

	l := loader.New(emptyUserRepo(), emptyRoleRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), prefRepo)
	_, errs := loadAllUserPreferences(context.Background(), l, []string{"x", "y", "z"})

	for i, err := range errs {
		if !errors.Is(err, wantErr) {
			t.Fatalf("load %d: want %v, got %v", i, wantErr, err)
		}
	}
}

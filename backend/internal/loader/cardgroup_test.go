package loader_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"backend/internal/domain"
	"backend/internal/loader"
)

// loadAllCardgroups concurrently loads all ids through l.Cardgroup and returns
// aligned results/errors.
func loadAllCardgroups(ctx context.Context, l *loader.Loaders, ids []string) ([]*domain.Cardgroup, []error) {
	results := make([]*domain.Cardgroup, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.Cardgroup.Load(ctx, id)()
		}(i, id)
	}
	wg.Wait()
	return results, errs
}

func TestCardgroupLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	var receivedKeys []string
	cgRepo := &countingCardgroupRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.Cardgroup, error) {
			batchCalls.Add(1)
			receivedKeys = ids
			out := make(map[string]*domain.Cardgroup, len(ids))
			for _, id := range ids {
				out[id] = &domain.Cardgroup{ID: domain.CardgroupID(id), Name: domain.CardgroupName("cg-" + id)}
			}
			return out, nil
		},
	}

	// Build 100 distinct keys to maximise the chance the loader collapses them.
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("cg-%03d", i)
	}

	emptyUser := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	results, errs := loadAllCardgroups(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyUserRoleRepo(), cgRepo, emptyCardRepo(), emptyUserPreferenceRepo(), nil), ids)

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: unexpected error: %v", i, err)
		}
		if results[i] == nil {
			t.Fatalf("load %d: nil result", i)
		}
		if results[i].ID != domain.CardgroupID(ids[i]) {
			t.Fatalf("load %d: ID mismatch: got %q want %q", i, results[i].ID, ids[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("cardgroup BatchFunc should run exactly once, ran %d times", got)
	}
	if got := len(receivedKeys); got != 100 {
		t.Fatalf("batch should receive 100 keys, got %d", got)
	}
}

func TestCardgroupLoader_PartialNotFound(t *testing.T) {
	t.Parallel()

	cgRepo := &countingCardgroupRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.Cardgroup, error) {
			out := map[string]*domain.Cardgroup{}
			for _, id := range ids {
				if id == "missing" {
					continue
				}
				out[id] = &domain.Cardgroup{ID: domain.CardgroupID(id), Name: domain.CardgroupName("cg-" + id)}
			}
			return out, nil
		},
	}

	emptyUser := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	ids := []string{"present-1", "missing", "present-2"}
	results, errs := loadAllCardgroups(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyUserRoleRepo(), cgRepo, emptyCardRepo(), emptyUserPreferenceRepo(), nil), ids)

	if errs[0] != nil {
		t.Fatalf("present-1: unexpected error: %v", errs[0])
	}
	if results[0] == nil || results[0].ID != domain.CardgroupID("present-1") {
		t.Fatalf("present-1: bad result: %+v", results[0])
	}
	if errs[2] != nil {
		t.Fatalf("present-2: unexpected error: %v", errs[2])
	}
	if results[2] == nil || results[2].ID != domain.CardgroupID("present-2") {
		t.Fatalf("present-2: bad result: %+v", results[2])
	}
	if !errors.Is(errs[1], loader.ErrNotFound) {
		t.Fatalf("missing: want ErrNotFound, got %v", errs[1])
	}
	if results[1] != nil {
		t.Fatalf("missing: want nil result, got %+v", results[1])
	}
}

func TestCardgroupLoader_BatchFuncError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("cardgroup store exploded")
	cgRepo := &countingCardgroupRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Cardgroup, error) {
			return nil, wantErr
		},
	}

	emptyUser := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	ids := []string{"x", "y", "z"}
	_, errs := loadAllCardgroups(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyUserRoleRepo(), cgRepo, emptyCardRepo(), emptyUserPreferenceRepo(), nil), ids)

	for i, err := range errs {
		if !errors.Is(err, wantErr) {
			t.Fatalf("load %d: want %v, got %v", i, wantErr, err)
		}
	}
}

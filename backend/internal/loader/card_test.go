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

func loadAllCards(ctx context.Context, l *loader.Loaders, ids []string) ([]*domain.Card, []error) {
	results := make([]*domain.Card, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.Card.Load(ctx, id)()
		}(i, id)
	}
	wg.Wait()
	return results, errs
}

func TestCardLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	cardRepo := &countingCardRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.Card, error) {
			batchCalls.Add(1)
			out := make(map[string]*domain.Card, len(ids))
			for _, id := range ids {
				out[id] = &domain.Card{ID: id}
			}
			return out, nil
		},
	}
	emptyUser := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("card-%03d", i)
	}
	results, errs := loadAllCards(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), cardRepo, emptyUserPreferenceRepo(), nil), ids)

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: unexpected error: %v", i, err)
		}
		if results[i] == nil || results[i].ID != ids[i] {
			t.Fatalf("load %d: bad result %+v", i, results[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("card BatchFunc should run exactly once, ran %d times", got)
	}
}

func TestCardLoader_PartialNotFound(t *testing.T) {
	t.Parallel()

	cardRepo := &countingCardRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.Card, error) {
			out := map[string]*domain.Card{}
			for _, id := range ids {
				if id != "missing" {
					out[id] = &domain.Card{ID: id}
				}
			}
			return out, nil
		},
	}
	emptyUser := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	results, errs := loadAllCards(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyUserRoleRepo(), emptyCardgroupRepo(), cardRepo, emptyUserPreferenceRepo(), nil), []string{"present", "missing"})
	if errs[0] != nil || results[0] == nil || results[0].ID != "present" {
		t.Fatalf("present: result=%+v err=%v", results[0], errs[0])
	}
	if !errors.Is(errs[1], loader.ErrNotFound) {
		t.Fatalf("missing: want ErrNotFound, got %v", errs[1])
	}
	if results[1] != nil {
		t.Fatalf("missing: want nil result, got %+v", results[1])
	}
}

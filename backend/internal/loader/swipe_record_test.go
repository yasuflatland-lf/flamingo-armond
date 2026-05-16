package loader_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/loader"
	"backend/internal/repository"
)

type countingSwipeRecordRepo struct {
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.SwipeRecord, error)
}

func (r *countingSwipeRecordRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.SwipeRecord, error) {
	if r.findByIDs == nil {
		panic("countingSwipeRecordRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}

func (r *countingSwipeRecordRepo) FindByUserAndCardgroup(context.Context, string, string) ([]*domain.SwipeRecord, error) {
	panic("countingSwipeRecordRepo.FindByUserAndCardgroup not configured")
}

func (r *countingSwipeRecordRepo) ListRecentByUser(context.Context, string, int) ([]*domain.SwipeRecord, error) {
	panic("countingSwipeRecordRepo.ListRecentByUser not configured")
}

func (r *countingSwipeRecordRepo) CreateTx(context.Context, *gorm.DB, *domain.SwipeRecord) error {
	panic("countingSwipeRecordRepo.CreateTx not configured")
}

func loadAllSwipeRecords(ctx context.Context, l *loader.Loaders, ids []string) ([]*domain.SwipeRecord, []error) {
	results := make([]*domain.SwipeRecord, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.SwipeRecord.Load(ctx, id)()
		}(i, id)
	}
	wg.Wait()
	return results, errs
}

func TestSwipeRecordLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	swipeRepo := &countingSwipeRecordRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.SwipeRecord, error) {
			batchCalls.Add(1)
			out := make(map[string]*domain.SwipeRecord, len(ids))
			for _, id := range ids {
				out[id] = &domain.SwipeRecord{ID: id}
			}
			return out, nil
		},
	}
	emptyUser := &countingRepo{
		findByIDs: func(context.Context, []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	ids := make([]string, 20)
	for i := range ids {
		ids[i] = fmt.Sprintf("swipe-%03d", i)
	}
	results, errs := loadAllSwipeRecords(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), emptyUserPreferenceRepo(), swipeRepo), ids)

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: unexpected error: %v", i, err)
		}
		if results[i] == nil || results[i].ID != ids[i] {
			t.Fatalf("load %d: bad result %+v", i, results[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("swipe record BatchFunc should run exactly once, ran %d times", got)
	}
}

func TestSwipeRecordLoader_PartialNotFound(t *testing.T) {
	t.Parallel()

	swipeRepo := &countingSwipeRecordRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.SwipeRecord, error) {
			out := map[string]*domain.SwipeRecord{}
			for _, id := range ids {
				if id != "missing" {
					out[id] = &domain.SwipeRecord{ID: id}
				}
			}
			return out, nil
		},
	}
	emptyUser := &countingRepo{
		findByIDs: func(context.Context, []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	results, errs := loadAllSwipeRecords(context.Background(), loader.New(emptyUser, emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), emptyUserPreferenceRepo(), swipeRepo), []string{"present", "missing"})
	if errs[0] != nil || results[0] == nil || results[0].ID != "present" {
		t.Fatalf("present: result=%+v err=%v", results[0], errs[0])
	}
	if !errors.Is(errs[1], repository.ErrNotFound) {
		t.Fatalf("missing: want ErrNotFound, got %v", errs[1])
	}
	if results[1] != nil {
		t.Fatalf("missing: want nil result, got %+v", results[1])
	}
}

package loader_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backend/internal/domain"
	"backend/internal/loader"
	"gorm.io/gorm"
)

type countingUserCardFSRSRepo struct {
	findByUserAndCardIDs func(ctx context.Context, userID string, ids []string) (map[string]*domain.UserCardFSRS, error)
}

func (r *countingUserCardFSRSRepo) UpsertTx(_ context.Context, _ *gorm.DB, _ *domain.UserCardFSRS) error {
	panic("countingUserCardFSRSRepo.UpsertTx not configured")
}

func (r *countingUserCardFSRSRepo) FindByUserAndCardIDs(ctx context.Context, userID string, ids []string) (map[string]*domain.UserCardFSRS, error) {
	if r.findByUserAndCardIDs == nil {
		panic("countingUserCardFSRSRepo.FindByUserAndCardIDs not configured")
	}
	return r.findByUserAndCardIDs(ctx, userID, ids)
}

func emptyUserRepo() *countingRepo {
	return &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}
}

func loadAllUserCardFSRS(ctx context.Context, l *loader.Loaders, ids []string) ([]*domain.UserCardFSRS, []error) {
	results := make([]*domain.UserCardFSRS, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.UserCardFSRS.Load(ctx, id)()
		}(i, id)
	}
	wg.Wait()
	return results, errs
}

func TestUserCardFSRSLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	viewerID := "user-1"
	var batchCalls atomic.Int32
	var receivedUserID string
	var receivedKeys []string
	ucsRepo := &countingUserCardFSRSRepo{
		findByUserAndCardIDs: func(_ context.Context, userID string, ids []string) (map[string]*domain.UserCardFSRS, error) {
			batchCalls.Add(1)
			receivedUserID = userID
			receivedKeys = ids
			out := make(map[string]*domain.UserCardFSRS, len(ids))
			now := time.Now().UTC()
			for _, id := range ids {
				out[id] = domain.NewUserCardFSRSForNewCard(userID, id, now)
			}
			return out, nil
		},
	}

	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("card-%03d", i)
	}
	results, errs := loadAllUserCardFSRS(context.Background(), loader.NewWithUserCardFSRS(
		emptyUserRepo(), emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), nil, ucsRepo, viewerID,
	), ids)

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: unexpected error: %v", i, err)
		}
		if results[i] == nil || results[i].CardID != ids[i] || results[i].UserID != viewerID {
			t.Fatalf("load %d: bad result %+v", i, results[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("user-card-fsrs BatchFunc should run exactly once, ran %d times", got)
	}
	if receivedUserID != viewerID {
		t.Fatalf("batch userID = %q, want %q", receivedUserID, viewerID)
	}
	if got := len(receivedKeys); got != len(ids) {
		t.Fatalf("batch should receive %d keys, got %d", len(ids), got)
	}
}

func TestUserCardFSRSLoader_MissingRowsReturnNilData(t *testing.T) {
	t.Parallel()

	ucsRepo := &countingUserCardFSRSRepo{
		findByUserAndCardIDs: func(_ context.Context, userID string, ids []string) (map[string]*domain.UserCardFSRS, error) {
			out := map[string]*domain.UserCardFSRS{}
			now := time.Now().UTC()
			for _, id := range ids {
				if id != "missing" {
					out[id] = domain.NewUserCardFSRSForNewCard(userID, id, now)
				}
			}
			return out, nil
		},
	}

	results, errs := loadAllUserCardFSRS(context.Background(), loader.NewWithUserCardFSRS(
		emptyUserRepo(), emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo(), nil, ucsRepo, "user-1",
	), []string{"present", "missing"})
	if errs[0] != nil || results[0] == nil || results[0].CardID != "present" {
		t.Fatalf("present: result=%+v err=%v", results[0], errs[0])
	}
	if errs[1] != nil {
		t.Fatalf("missing: unexpected error: %v", errs[1])
	}
	if results[1] != nil {
		t.Fatalf("missing: want nil result, got %+v", results[1])
	}
}

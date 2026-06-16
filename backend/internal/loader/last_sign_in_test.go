package loader_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/internal/loader"
	"backend/internal/repository"
)

func newLoadersForLastSignIn(userRepo repository.UserRepository) *loader.Loaders {
	return loader.New(
		userRepo,
		emptyRoleRepo(),
		emptyUserRoleRepo(),
		emptyCardgroupRepo(),
		emptyCardRepo(),
		emptyUserPreferenceRepo(),
	)
}

func TestLastSignInByUserIDLoader_ReturnsTimestampNilAndUnknown(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	repo := &countingRepo{
		lastSignInByUserIDs: func(_ context.Context, _ []string) (map[string]*time.Time, error) {
			// "u-1" signed in; "u-2" exists but never signed in (nil); any
			// other id is omitted entirely (unknown user).
			return map[string]*time.Time{"u-1": &ts, "u-2": nil}, nil
		},
	}

	l := newLoadersForLastSignIn(repo)
	if l.LastSignInByUserID == nil {
		t.Fatal("LastSignInByUserID loader was not wired")
	}

	got1, err := l.LastSignInByUserID.Load(context.Background(), "u-1")()
	if err != nil {
		t.Fatalf("Load u-1: %v", err)
	}
	if got1 == nil || !got1.Equal(ts) {
		t.Errorf("u-1 = %v, want %v", got1, ts)
	}

	got2, err := l.LastSignInByUserID.Load(context.Background(), "u-2")()
	if err != nil {
		t.Fatalf("Load u-2: %v", err)
	}
	if got2 != nil {
		t.Errorf("u-2 = %v, want nil (never signed in)", got2)
	}

	got3, err := l.LastSignInByUserID.Load(context.Background(), "u-unknown")()
	if err != nil {
		t.Fatalf("Load u-unknown: %v", err)
	}
	if got3 != nil {
		t.Errorf("u-unknown = %v, want nil (unknown user is not an error)", got3)
	}
}

func TestLastSignInByUserIDLoader_BatchFuncErrorWraps(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db blew up")
	repo := &countingRepo{
		lastSignInByUserIDs: func(_ context.Context, _ []string) (map[string]*time.Time, error) {
			return nil, wantErr
		},
	}

	l := newLoadersForLastSignIn(repo)
	_, err := l.LastSignInByUserID.Load(context.Background(), "u1")()
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("want errors.Is(err, wantErr) == true; got chain: %v", err)
	}
}

package usecase

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// --- fakes -----------------------------------------------------------------

type fakeMasterCGRepo struct {
	byID         map[string]*domain.MasterCardgroup
	findErr      error
	starters     []*domain.MasterCardgroup
	startersErr  error
	findCalls    int
	startersCall int
}

func (f *fakeMasterCGRepo) FindByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	f.findCalls++
	if f.findErr != nil {
		return nil, f.findErr
	}
	m, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return m, nil
}

func (f *fakeMasterCGRepo) ListDefaultStarters(_ context.Context) ([]*domain.MasterCardgroup, error) {
	f.startersCall++
	return f.starters, f.startersErr
}

type fakeMasterCardRepo struct {
	byMaster map[string][]*domain.MasterCard
	listErr  error
}

func (f *fakeMasterCardRepo) ListByMasterCardgroup(_ context.Context, masterID string) ([]*domain.MasterCard, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byMaster[masterID], nil
}

type fakeUserCardRepo struct {
	// captured holds a deep copy of every card batch handed to UpsertManyTx so
	// assertions survive any caller-side mutation.
	captured   [][]*domain.Card
	upsertErr  error
	upsertCall int
}

func (f *fakeUserCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error) {
	f.upsertCall++
	batch := make([]*domain.Card, len(cards))
	for i, c := range cards {
		clone := *c
		batch[i] = &clone
	}
	f.captured = append(f.captured, batch)
	if f.upsertErr != nil {
		return repository.UpsertManyTxResult{}, f.upsertErr
	}
	return repository.UpsertManyTxResult{Inserted: int64(len(cards))}, nil
}

// fakeUserCG implements masterDeckUserCardgroupRepo: the idempotency-guard
// CountByOwner plus the snapshot-cardgroup CreateTx. createdCGs records every
// cardgroup handed to CreateTx (deep-copied) so assertions survive caller-side
// mutation.
type fakeUserCG struct {
	count       int64
	countErr    error
	calls       int
	lastUser    string
	createErr   error
	createCalls int
	createdCGs  []*domain.Cardgroup
}

func (f *fakeUserCG) CountByOwner(_ context.Context, ownerID string, _ *string) (int64, error) {
	f.calls++
	f.lastUser = ownerID
	return f.count, f.countErr
}

func (f *fakeUserCG) CreateTx(_ context.Context, _ *gorm.DB, cg *domain.Cardgroup) error {
	f.createCalls++
	clone := *cg
	f.createdCGs = append(f.createdCGs, &clone)
	if f.createErr != nil {
		return f.createErr
	}
	return nil
}

// --- in-memory tx harness --------------------------------------------------

// fakeResult is a no-op sql.Result for the recording ConnPool.
type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

// recordPool is a gorm.ConnPool that records ExecContext / QueryContext SQL
// without a real database connection. Paired with WithoutReturning so the
// cardgroup INSERT goes through ExecContext (no RETURNING → no *sql.Rows to
// fabricate). This lets the production copy path run end-to-end — the inline
// tx.Create for the cardgroup and the advisory-lock Exec both execute against
// the recorder, while the card batch is captured at the userCard mock boundary.
type recordPool struct {
	sqls []string
	args [][]any
}

func (p *recordPool) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error) { return nil, nil }

func (p *recordPool) ExecContext(_ context.Context, q string, args ...any) (sql.Result, error) {
	p.sqls = append(p.sqls, q)
	p.args = append(p.args, args)
	return fakeResult{}, nil
}

func (p *recordPool) QueryContext(_ context.Context, q string, _ ...any) (*sql.Rows, error) {
	p.sqls = append(p.sqls, q)
	return nil, nil
}

func (p *recordPool) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row { return nil }

// newRecordingDB returns a *gorm.DB whose ConnPool records statements instead of
// touching a database, plus the recorder for assertions.
func newRecordingDB(t *testing.T) (*gorm.DB, *recordPool) {
	t.Helper()
	pool := &recordPool{}
	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: pool, WithoutQuotingCheck: true, WithoutReturning: true}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, pool
}

// recordingTxRunner returns a txRunner that runs fn against a recording *gorm.DB
// session, plus the recorder and a pointer to the number of times the runner was
// invoked.
func recordingTxRunner(t *testing.T) (txRunner, *recordPool, *int) {
	t.Helper()
	db, pool := newRecordingDB(t)
	calls := 0
	runner := func(ctx context.Context, fn func(tx *gorm.DB) error) error {
		calls++
		return fn(db.WithContext(ctx))
	}
	return runner, pool, &calls
}

// --- fixtures --------------------------------------------------------------

func masterCG(id, name string) *domain.MasterCardgroup {
	return &domain.MasterCardgroup{
		ID:               id,
		Name:             domain.CardgroupName(name),
		Status:           domain.MasterStatusPublished,
		IsDefaultStarter: true,
	}
}

func masterCard(id, masterID, front, back string, pos int) *domain.MasterCard {
	return &domain.MasterCard{
		ID:                id,
		MasterCardgroupID: masterID,
		Front:             domain.CardText(front),
		Back:              domain.CardText(back),
		Position:          pos,
	}
}

func newSeedUsecase(
	t *testing.T,
	cg masterDeckCardgroupRepo,
	card masterDeckCardRepo,
	user masterDeckUserCardRepo,
	userCG masterDeckUserCardgroupRepo,
) (*masterDeckUsecase, *recordPool, *int) {
	t.Helper()
	runner, pool, calls := recordingTxRunner(t)
	uc := NewMasterDeckUsecaseWithTx(cg, card, user, userCG, runner, newTestLogger())
	return uc, pool, calls
}

// --- constructor panic tests -----------------------------------------------

func TestNewMasterDeckUsecase_PanicsOnNilDeps(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}
	logger := newTestLogger()
	db, _ := newRecordingDB(t)

	cases := []struct {
		name string
		fn   func()
	}{
		{"nil masterCG", func() { NewMasterDeckUsecase(nil, card, user, userCG, db, logger) }},
		{"nil masterCard", func() { NewMasterDeckUsecase(cg, nil, user, userCG, db, logger) }},
		{"nil userCard", func() { NewMasterDeckUsecase(cg, card, nil, userCG, db, logger) }},
		{"nil userCG", func() { NewMasterDeckUsecase(cg, card, user, nil, db, logger) }},
		{"nil db", func() { NewMasterDeckUsecase(cg, card, user, userCG, nil, logger) }},
		{"nil logger", func() { NewMasterDeckUsecase(cg, card, user, userCG, db, nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, tc.fn)
		})
	}
}

func TestNewMasterDeckUsecaseWithTx_PanicsOnNilDeps(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}
	logger := newTestLogger()
	runner := func(ctx context.Context, fn func(tx *gorm.DB) error) error { return nil }

	cases := []struct {
		name string
		fn   func()
	}{
		{"nil masterCG", func() { NewMasterDeckUsecaseWithTx(nil, card, user, userCG, runner, logger) }},
		{"nil masterCard", func() { NewMasterDeckUsecaseWithTx(cg, nil, user, userCG, runner, logger) }},
		{"nil userCard", func() { NewMasterDeckUsecaseWithTx(cg, card, nil, userCG, runner, logger) }},
		{"nil userCG", func() { NewMasterDeckUsecaseWithTx(cg, card, user, nil, runner, logger) }},
		{"nil tx", func() { NewMasterDeckUsecaseWithTx(cg, card, user, userCG, nil, logger) }},
		{"nil logger", func() { NewMasterDeckUsecaseWithTx(cg, card, user, userCG, runner, nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, tc.fn)
		})
	}
}

// --- CopyMasterToUser tests ------------------------------------------------

func TestCopyMasterToUser_CopiesContentWithFreshIDsAndPositions(t *testing.T) {
	t.Parallel()

	const masterID = "m1"
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Starter Deck")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {
			masterCard("mc1", masterID, "front-1", "back-1", 0),
			masterCard("mc2", masterID, "front-2", "back-2", 1),
			masterCard("mc3", masterID, "front-3", "back-3", 2),
		},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, calls := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-1")
	require.NoError(t, err)
	require.Equal(t, 1, *calls, "should run in exactly one transaction")

	// New cardgroup: fresh id (UUID v7), owner set, name copied from master.
	require.NotNil(t, got)
	assert.NotEmpty(t, got.ID)
	assert.Equal(t, "owner-1", got.OwnerID)
	assert.Equal(t, domain.CardgroupName("Starter Deck"), got.Name)

	require.Len(t, user.captured, 1)
	copied := user.captured[0]
	require.Len(t, copied, 3)

	for i, c := range copied {
		src := card.byMaster[masterID][i]
		// Content identical (front/back/position preserved in order).
		assert.Equal(t, src.Front, c.Front, "front preserved")
		assert.Equal(t, src.Back, c.Back, "back preserved")
		assert.Equal(t, src.Position, c.Position, "position preserved")
		// Fresh card id, distinct from the master card's id.
		assert.NotEmpty(t, c.ID)
		assert.NotEqual(t, src.ID, c.ID, "card id is freshly generated")
		// Reparented to the new cardgroup.
		assert.Equal(t, got.ID, c.CardgroupID, "card points at the new cardgroup")
	}

	// FSRS/swipe state is empty by construction: the copy path writes nothing to
	// the per-user FSRS table — only userCard.UpsertManyTx is invoked.
	assert.Equal(t, 1, user.upsertCall)
}

func TestCopyMasterToUser_EmptyDeck_CopiesEmptyCardgroup(t *testing.T) {
	t.Parallel()

	const masterID = "m-empty"
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Empty Deck")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{}} // no cards
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, calls := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-2")
	require.NoError(t, err)
	require.Equal(t, 1, *calls)

	require.NotNil(t, got)
	assert.NotEmpty(t, got.ID)
	assert.Equal(t, domain.CardgroupName("Empty Deck"), got.Name)

	// UpsertManyTx is still invoked exactly once, with an empty (non-nil) batch.
	assert.Equal(t, 1, user.upsertCall, "the bulk insert runs once even for an empty deck")
	require.Len(t, user.captured, 1)
	assert.Empty(t, user.captured[0])
}

func TestCopyMasterToUser_MasterNotFound_ReturnsInternalChain(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{}} // master absent
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), "missing", "owner-3")
	require.Error(t, err)
	assert.Nil(t, got)
	// Caller prefix is applied by CopyMasterToUser, not by the shared helper.
	assertInternalChain(t, err, "usecase: master deck: copy master to user")
	assert.Empty(t, user.captured, "no cards persisted when the master lookup fails")
}

// --- SeedForNewUser tests --------------------------------------------------

func TestSeedForNewUser_CopiesAllDefaultStarters(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{
		byID: map[string]*domain.MasterCardgroup{
			"m1": masterCG("m1", "Deck One"),
			"m2": masterCG("m2", "Deck Two"),
		},
		starters: []*domain.MasterCardgroup{masterCG("m1", "Deck One"), masterCG("m2", "Deck Two")},
	}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		"m1": {masterCard("a1", "m1", "f1", "b1", 0)},
		"m2": {masterCard("a2", "m2", "f2", "b2", 0), masterCard("a3", "m2", "f3", "b3", 1)},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0} // brand-new user

	uc, _, calls := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "new-user")
	require.NoError(t, err)
	require.Equal(t, 1, *calls, "the whole batch runs in a single transaction")

	// Idempotency guard consulted once with the seeded user id.
	assert.Equal(t, 1, userCG.calls)
	assert.Equal(t, "new-user", userCG.lastUser)

	// One copy per default starter; each copies its own cards.
	require.Len(t, user.captured, 2)
	assert.Len(t, user.captured[0], 1) // Deck One
	assert.Len(t, user.captured[1], 2) // Deck Two

	// Every copied card is reparented and carries a fresh id.
	for _, batch := range user.captured {
		for _, c := range batch {
			assert.NotEmpty(t, c.ID)
			assert.NotEmpty(t, c.CardgroupID)
		}
	}
}

func TestSeedForNewUser_SecondCall_NoOps(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{
		byID:     map[string]*domain.MasterCardgroup{"m1": masterCG("m1", "Deck One")},
		starters: []*domain.MasterCardgroup{masterCG("m1", "Deck One")},
	}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{"m1": {masterCard("a1", "m1", "f", "b", 0)}}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 3} // user already owns cardgroups

	uc, _, calls := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "returning-user")
	require.NoError(t, err)
	require.Equal(t, 1, *calls, "the guard still runs inside a transaction (advisory lock)")

	// Guard short-circuits before listing starters or copying anything.
	assert.Equal(t, 1, userCG.calls)
	assert.Equal(t, 0, cg.startersCall, "default starters are not listed when the guard trips")
	assert.Equal(t, 0, cg.findCalls, "no master deck is looked up on the no-op path")
	assert.Equal(t, 0, userCG.createCalls, "no user cardgroup is inserted on the no-op path")
	assert.Empty(t, user.captured, "no cards persisted on the no-op path")
}

func TestSeedForNewUser_TakesAdvisoryLockBeforeCounting(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{starters: nil}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 5} // short-circuit after the lock

	uc, pool, _ := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "lock-user")
	require.NoError(t, err)

	// The first statement executed against the tx is the transaction-scoped
	// advisory lock; the idempotency COUNT (mocked) runs after it.
	require.NotEmpty(t, pool.sqls)
	assert.Contains(t, pool.sqls[0], "pg_advisory_xact_lock(0, hashtext(")
	// The lock is keyed on the user id: the single bound argument is the user,
	// so two distinct users never contend on the same advisory lock.
	require.NotEmpty(t, pool.args)
	require.Len(t, pool.args[0], 1)
	assert.Equal(t, "lock-user", pool.args[0][0])
}

func TestSeedForNewUser_NoStarters_NoCopies(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{starters: nil}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "no-starter-user")
	require.NoError(t, err)
	assert.Equal(t, 1, cg.startersCall)
	assert.Empty(t, user.captured)
}

func TestSeedForNewUser_CountError_Propagates(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{countErr: errors.New("db down")}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "err-user")
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: seed for new user")
	assert.Empty(t, user.captured)
}

func TestSeedForNewUser_ListStartersError_PropagatesChain(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{startersErr: errors.New("starters query failed")}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0} // brand-new user → guard passes, then list fails

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "starter-err-user")
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: seed for new user")
	assert.Equal(t, 1, cg.startersCall, "ListDefaultStarters was attempted")
	assert.Empty(t, user.captured, "no cards persisted when listing starters fails")
}

func TestSeedForNewUser_MidLoopCopyFailure_AbortsBatch(t *testing.T) {
	t.Parallel()

	// Two default starters: the copy of m1 succeeds, m2's master lookup fails.
	cg := &fakeMasterCGRepo{
		byID:     map[string]*domain.MasterCardgroup{"m1": masterCG("m1", "Deck One")},
		starters: []*domain.MasterCardgroup{masterCG("m1", "Deck One"), masterCG("m2", "Deck Two")},
	}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		"m1": {masterCard("a1", "m1", "f1", "b1", 0)},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "mid-loop-user")
	// The loop aborts on the second starter (m2 is absent from byID, so FindByID
	// returns ErrNotFound). The whole batch fails. The real transaction rollback
	// is exercised by the integration test; here we prove the loop stops.
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: seed for new user")
	// Deck One copied before the failure; Deck Two never reached the card insert.
	require.Len(t, user.captured, 1, "only the first starter was copied before the abort")
}

func TestSeedForNewUser_ContextCancelled_PassesThrough(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{countErr: context.Canceled}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	err := uc.SeedForNewUser(context.Background(), "cancelled-user")
	require.Error(t, err)
	assertCancelled(t, err)
}

func TestCopyMasterToUser_ContextCancelled_PassesThrough(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{findErr: context.Canceled}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), "any-master", "owner-cancel")
	require.Error(t, err)
	assert.Nil(t, got)
	assertCancelled(t, err)
}

func TestCopyMasterToUser_ListCardsError_ReturnsInternalChain(t *testing.T) {
	t.Parallel()

	const masterID = "m-listerr"
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Deck")}}
	card := &fakeMasterCardRepo{listErr: errors.New("list cards failed")}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-listerr")
	require.Error(t, err)
	assert.Nil(t, got)
	assertInternalChain(t, err, "usecase: master deck: copy master to user")
	assert.Equal(t, 0, userCG.createCalls, "no cardgroup is inserted when listing cards fails")
	assert.Empty(t, user.captured)
}

func TestCopyMasterToUser_UpsertCardsError_PropagatesChain(t *testing.T) {
	t.Parallel()

	const masterID = "m-upserterr"
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Deck")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {masterCard("mc1", masterID, "f1", "b1", 0)},
	}}
	user := &fakeUserCardRepo{upsertErr: errors.New("upsert failed")}
	userCG := &fakeUserCG{}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-upserterr")
	require.Error(t, err)
	assert.Nil(t, got)
	assertInternalChain(t, err, "usecase: master deck: copy master to user")
	// The bulk insert was attempted (one batch captured) before the error.
	assert.Equal(t, 1, user.upsertCall, "the card insert was attempted")
	require.Len(t, user.captured, 1)
}

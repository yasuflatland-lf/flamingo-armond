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
	"backend/internal/usecase/ucerr"
)

// --- fakes -----------------------------------------------------------------

type fakeMasterCGRepo struct {
	byID    map[string]*domain.MasterCardgroup
	findErr error
	// findErrOnCall is 1-based; 0 means findErr applies to every call. Set it to
	// target one iteration of a loop that reads the published master per starter.
	// The counter it keys on spans both the pooled and the tx-scoped read.
	findErrOnCall int
	starters      []*domain.MasterCardgroup
	startersErr   error
	findCalls     int
	startersCall  int

	// pooledCalls counts FindPublishedByID; txHandles records the *gorm.DB handed
	// to each FindPublishedByIDTx call. Together they let a test prove a write
	// path probed on its own transaction rather than on a pooled connection.
	pooledCalls int
	txHandles   []*gorm.DB
}

// FindPublishedByID models the published-scoped master read on a pooled
// connection — the read-only preview path. byID represents the currently-published
// set: an id absent from the map returns repository.ErrNotFound, which is how a
// test simulates a master that was unpublished after the caller's gate passed
// (the TOCTOU regression case).
func (f *fakeMasterCGRepo) FindPublishedByID(_ context.Context, id string) (*domain.MasterCardgroup, error) {
	f.pooledCalls++
	return f.findPublished(id)
}

// FindPublishedByIDTx models the transaction-scoped, FOR SHARE-locked read the
// write paths take. It answers from the same published set as the pooled variant
// (mirroring the shared repository helper) and records the transaction handle it
// was given so a test can assert the probe ran on the write's own transaction.
func (f *fakeMasterCGRepo) FindPublishedByIDTx(_ context.Context, tx *gorm.DB, id string) (*domain.MasterCardgroup, error) {
	f.txHandles = append(f.txHandles, tx)
	return f.findPublished(id)
}

func (f *fakeMasterCGRepo) findPublished(id string) (*domain.MasterCardgroup, error) {
	f.findCalls++
	if f.findErr != nil && (f.findErrOnCall == 0 || f.findErrOnCall == f.findCalls) {
		return nil, f.findErr
	}
	m, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return m, nil
}

func (f *fakeMasterCGRepo) ListPublishedDefaultStarters(_ context.Context) ([]*domain.MasterCardgroup, error) {
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
	// result, when Inserted or Updated is non-zero, overrides the default
	// Inserted=len(cards) return. Used by merge tests to inject specific tallies.
	result repository.UpsertManyTxResult
	// existingFronts backs CountExistingFronts: maps cardgroupID -> front -> present.
	// Case-sensitive plain map lookup mirrors the text unique index the merge upserts against.
	existingFronts map[string]map[string]bool
	// countFrontsCalls counts CountExistingFronts so a preview test can assert the
	// tally step is skipped once the published probe rejects the deck.
	countFrontsCalls int
	// upsertTxHandles records the *gorm.DB each UpsertManyTx call received, so a
	// test can compare it against the handle the published probe was given.
	upsertTxHandles []*gorm.DB
}

func (f *fakeUserCardRepo) CountExistingFronts(_ context.Context, cardgroupID string, fronts []string) (int64, error) {
	f.countFrontsCalls++
	present := f.existingFronts[cardgroupID]
	var n int64
	for _, fr := range fronts {
		if present[fr] {
			n++
		}
	}
	return n, nil
}

func (f *fakeUserCardRepo) UpsertManyTx(_ context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error) {
	f.upsertCall++
	f.upsertTxHandles = append(f.upsertTxHandles, tx)
	batch := make([]*domain.Card, len(cards))
	for i, c := range cards {
		clone := *c
		batch[i] = &clone
	}
	f.captured = append(f.captured, batch)
	if f.upsertErr != nil {
		return repository.UpsertManyTxResult{}, f.upsertErr
	}
	if f.result.Inserted != 0 || f.result.Updated != 0 {
		return f.result, nil
	}
	return repository.UpsertManyTxResult{Inserted: int64(len(cards))}, nil
}

// fakeUserCG implements masterDeckUserCardgroupRepo: the idempotency-guard
// CountByOwner plus the snapshot-cardgroup CreateTx. createdCGs records every
// cardgroup handed to CreateTx (deep-copied) so assertions survive caller-side
// mutation. byID backs FindByID for ownership checks and post-merge result
// retrieval; a missing key returns repository.ErrNotFound.
//
// For tests that need FindByID to succeed on call 1 (ownership gate) but fail
// on a later call (e.g. post-tx read), set findByIDErr and findByIDErrOnCall
// to the 1-based call number at which the error should be injected. A zero
// findByIDErrOnCall never injects (the default for all existing tests).
type fakeUserCG struct {
	count             int64
	countErr          error
	calls             int
	lastUser          string
	createErr         error
	createCalls       int
	createdCGs        []*domain.Cardgroup
	byID              map[string]*domain.Cardgroup
	findByIDCalls     int
	findByIDErr       error
	findByIDErrOnCall int // 1-based; 0 = never inject
	lockCalls         int
	lockUser          string
	lockCountAtCall   int // snapshot of CountByOwner calls when the lock was taken (0 ⇒ lock before count)
}

func (f *fakeUserCG) FindByID(_ context.Context, id string) (*domain.Cardgroup, error) {
	f.findByIDCalls++
	if f.findByIDErrOnCall != 0 && f.findByIDCalls == f.findByIDErrOnCall {
		return nil, f.findByIDErr
	}
	cg, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return cg, nil
}

func (f *fakeUserCG) CountByOwner(_ context.Context, ownerID string, _ *string) (int64, error) {
	f.calls++
	f.lastUser = ownerID
	return f.count, f.countErr
}

// AcquireUserSeedLockTx records that the per-user advisory lock was taken and
// snapshots how many CountByOwner calls had run at that point, so the seed
// usecase test can assert the lock is taken before the idempotency count.
func (f *fakeUserCG) AcquireUserSeedLockTx(_ context.Context, _ *gorm.DB, userID string) error {
	f.lockCalls++
	f.lockUser = userID
	f.lockCountAtCall = f.calls
	return nil
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
// tx.Create for the cardgroup executes against the recorder, while the card
// batch is captured at the userCard mock boundary. (The advisory lock now lives
// in the repository, so it is recorded at the fakeUserCG boundary instead.)
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
	uc := newMasterDeckUsecaseWithTx(cg, card, user, userCG, runner, newTestLogger())
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

func Test_newMasterDeckUsecaseWithTx_PanicsOnNilDeps(t *testing.T) {
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
		{"nil masterCG", func() { newMasterDeckUsecaseWithTx(nil, card, user, userCG, runner, logger) }},
		{"nil masterCard", func() { newMasterDeckUsecaseWithTx(cg, nil, user, userCG, runner, logger) }},
		{"nil userCard", func() { newMasterDeckUsecaseWithTx(cg, card, nil, userCG, runner, logger) }},
		{"nil userCG", func() { newMasterDeckUsecaseWithTx(cg, card, user, nil, runner, logger) }},
		{"nil tx", func() { newMasterDeckUsecaseWithTx(cg, card, user, userCG, nil, logger) }},
		{"nil logger", func() { newMasterDeckUsecaseWithTx(cg, card, user, userCG, runner, nil) }},
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
	assert.Equal(t, domain.UserID("owner-1"), got.OwnerID)
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

// TestCopyMasterToUser_EmptyDeck_ReturnsNotFoundWithoutWriting pins the
// emptiness half of catalog visibility at the copy. The FindPublishedByID probe
// and the card enumeration do not share a snapshot — neither takes the tx handle
// — so a deck whose last card is deleted between them would pass the probe and
// then write a cardgroup with no cards. Deciding emptiness off the enumeration
// this copy actually consumes makes that unreachable: the copy collapses into
// repository.ErrNotFound, which ImportMaster maps to its non-disclosure
// not-found outcome, and no user rows are written.
func TestCopyMasterToUser_EmptyDeck_ReturnsNotFoundWithoutWriting(t *testing.T) {
	t.Parallel()

	const masterID = "m-empty"
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Empty Deck")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{}} // no cards
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, calls := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-2")
	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrNotFound,
		"an empty deck must collapse into the same not-found the catalog uses for an unknown id")
	assert.Nil(t, got)
	require.Equal(t, 1, *calls)

	// Nothing is persisted: neither the cardgroup row nor the (empty) card batch.
	assert.Zero(t, user.upsertCall, "no card batch is written for a deck that left the catalog")
	assert.Empty(t, user.captured)
	assert.Zero(t, userCG.createCalls, "no cardgroup row is created for a deck that left the catalog")
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

	seeded, err := uc.SeedForNewUser(context.Background(), "new-user")
	require.NoError(t, err)
	require.Equal(t, 1, *calls, "the whole batch runs in a single transaction")
	require.Len(t, seeded, 2, "one cardgroup per default starter")

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

	seeded, err := uc.SeedForNewUser(context.Background(), "returning-user")
	require.NoError(t, err)
	require.Equal(t, 1, *calls, "the guard still runs inside a transaction (advisory lock)")
	require.NotNil(t, seeded, "no-op path returns a non-nil empty slice, not nil")
	require.Empty(t, seeded, "no cardgroups seeded on no-op path")

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

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	_, err := uc.SeedForNewUser(context.Background(), "lock-user")
	require.NoError(t, err)

	// The advisory lock (now owned by the repository) is taken exactly once and
	// before the idempotency COUNT: lockCountAtCall snapshots the CountByOwner
	// call counter at lock time, so 0 proves the lock came first.
	assert.Equal(t, 1, userCG.lockCalls, "the advisory lock is taken once")
	assert.Equal(t, 0, userCG.lockCountAtCall, "the lock is taken before the idempotency count")
	// The lock is keyed on the user id, so two distinct users never contend on
	// the same advisory lock.
	assert.Equal(t, "lock-user", userCG.lockUser)
}

func TestSeedForNewUser_NoStarters_NoCopies(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{starters: nil}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	seeded, err := uc.SeedForNewUser(context.Background(), "no-starter-user")
	require.NoError(t, err)
	require.NotNil(t, seeded, "no-defaults path returns a non-nil empty slice, not nil")
	require.Empty(t, seeded, "no cardgroups seeded when no default starters exist")
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

	_, err := uc.SeedForNewUser(context.Background(), "err-user")
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

	_, err := uc.SeedForNewUser(context.Background(), "starter-err-user")
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: seed for new user")
	assert.Equal(t, 1, cg.startersCall, "ListPublishedDefaultStarters was attempted")
	assert.Empty(t, user.captured, "no cards persisted when listing starters fails")
}

// TestSeedForNewUser_StarterLeftCatalog_SkipsAndSeedsTheRest pins the skip half
// of the seed contract. A starter can leave the catalog between
// ListPublishedDefaultStarters and its copy — unpublished, deleted, or emptied
// of its last card — and all three surface as repository.ErrNotFound. Failing
// the whole seed would turn a rare admin action into a broken signup, so the
// loop skips that starter and seeds the rest. The guarantee being protected is
// "never seed an empty deck", not "seed every listed starter".
func TestSeedForNewUser_StarterLeftCatalog_SkipsAndSeedsTheRest(t *testing.T) {
	t.Parallel()

	// Two default starters: m1 copies fine; m2 is absent from byID, so its
	// catalog-scoped lookup returns ErrNotFound.
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

	seeded, err := uc.SeedForNewUser(context.Background(), "mid-loop-user")
	require.NoError(t, err, "one starter leaving the catalog must not fail the whole seed")
	require.Len(t, seeded, 1, "only the still-visible starter is seeded")
	assert.Equal(t, domain.CardgroupName("Deck One"), seeded[0].Name)
	require.Len(t, user.captured, 1, "the skipped starter writes no cards")
}

// TestSeedForNewUser_StarterEmptied_IsSkipped is the emptiness-specific case of
// the skip above: the starter is still present and published, but its card
// enumeration comes back empty. The copy's len(cards) guard turns that into the
// same ErrNotFound, so the learner is never handed an empty deck.
func TestSeedForNewUser_StarterEmptied_IsSkipped(t *testing.T) {
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
		"m2": {}, // emptied between the listing and the copy
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	seeded, err := uc.SeedForNewUser(context.Background(), "emptied-starter-user")
	require.NoError(t, err)
	require.Len(t, seeded, 1, "the emptied starter is skipped, the other is seeded")
	assert.Equal(t, domain.CardgroupName("Deck One"), seeded[0].Name)
	assert.Equal(t, 1, userCG.createCalls, "no cardgroup row is created for the emptied starter")
}

// TestSeedForNewUser_MidLoopInfraFailure_AbortsBatch pins the other half of the
// contract: only the catalog-visibility sentinel is skippable. An infrastructure
// failure mid-loop still aborts the whole batch, so a database fault is never
// silently downgraded into a partial seed. The real transaction rollback is
// exercised by the integration test; here we prove the loop stops.
func TestSeedForNewUser_MidLoopInfraFailure_AbortsBatch(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	cg := &fakeMasterCGRepo{
		byID:     map[string]*domain.MasterCardgroup{"m1": masterCG("m1", "Deck One")},
		starters: []*domain.MasterCardgroup{masterCG("m1", "Deck One"), masterCG("m2", "Deck Two")},
	}
	card := &fakeMasterCardRepo{
		byMaster: map[string][]*domain.MasterCard{"m1": {masterCard("a1", "m1", "f1", "b1", 0)}},
	}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{count: 0}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)
	// Fail the SECOND starter's master lookup with a non-sentinel error.
	cg.findErrOnCall = 2
	cg.findErr = boom

	_, err := uc.SeedForNewUser(context.Background(), "infra-failure-user")
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: seed for new user")
	require.Len(t, user.captured, 1, "only the first starter was copied before the abort")
}

func TestSeedForNewUser_ContextCancelled_PassesThrough(t *testing.T) {
	t.Parallel()

	cg := &fakeMasterCGRepo{}
	card := &fakeMasterCardRepo{}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{countErr: context.Canceled}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	_, err := uc.SeedForNewUser(context.Background(), "cancelled-user")
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

func TestCopyMasterToUser_CreateCGError_PropagatesChain(t *testing.T) {
	t.Parallel()

	const masterID = "m-createerr"
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Deck")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {masterCard("mc1", masterID, "f1", "b1", 0)},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{createErr: errors.New("db constraint violation")}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	got, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-createerr")
	require.Error(t, err)
	assert.Nil(t, got)
	assertInternalChain(t, err, "usecase: master deck: copy master to user")
	// CreateTx was attempted once before failing.
	assert.Equal(t, 1, userCG.createCalls, "CreateTx was attempted")
	// No cards are inserted when the cardgroup creation fails.
	assert.Empty(t, user.captured, "no cards persisted when cardgroup creation fails")
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

// --- MergeMasterIntoCardgroup helpers and tests ----------------------------

// stubTxRunner is a minimal txRunner for merge tests: it calls fn with a nil
// *gorm.DB. All fakes ignore the *gorm.DB argument so nil is safe here.
var stubTxRunner = txRunner(func(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
})

// mustCardgroup builds a *domain.Cardgroup directly from the given IDs and name,
// bypassing NewCardgroup's ID generator. Use only in tests where a deterministic
// ID is required (e.g. to pre-populate the byID map in fakeUserCG).
func mustCardgroup(t *testing.T, id, ownerID, name string) *domain.Cardgroup {
	t.Helper()
	return &domain.Cardgroup{
		ID:      domain.CardgroupID(id),
		OwnerID: domain.UserID(ownerID),
		Name:    domain.CardgroupName(name),
	}
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_AddsAndUpdates(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	// Two master cards to merge.
	masterCards := []*domain.MasterCard{
		masterCard("mc-1", masterID, "alpha", "first", 0),
		masterCard("mc-2", masterID, "beta", "second", 1),
	}
	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}}, // published-scoped re-read (runs outside the tx)
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{masterID: masterCards}},
		&fakeUserCardRepo{result: repository.UpsertManyTxResult{Inserted: 1, Updated: 1}},
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner,
		newTestLogger(),
	)

	res, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, int64(1), res.Added)
	assert.Equal(t, int64(1), res.Updated)
	assert.Equal(t, domain.CardgroupID(destID), res.Cardgroup.ID)
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_NotOwned_Unauthenticated(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const otherID = "99999999-9999-7999-8999-999999999999"
	const destID = "22222222-2222-7222-8222-222222222222"
	destCG := mustCardgroup(t, destID, otherID, "Not Mine")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{}, &fakeMasterCardRepo{}, &fakeUserCardRepo{},
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner, newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), "master-id", domain.CardgroupID(destID), domain.UserID(ownerID))
	require.ErrorIs(t, err, ucerr.ErrUnauthenticated)
}

// TestMasterDeckUsecase_MergeMasterIntoCardgroup_PublishedProbeRunsOnTxHandle
// pins the wiring the unpublish fix rests on: the merge's published probe goes
// through FindPublishedByIDTx on the SAME *gorm.DB the card upsert writes
// through, never through the pooled FindPublishedByID. Handle identity is what
// puts the repository's FOR SHARE lock inside the write's transaction; a probe on
// a pooled connection would release its lock immediately and leave the window
// open, while still passing every state-based assertion in this file.
func TestMasterDeckUsecase_MergeMasterIntoCardgroup_PublishedProbeRunsOnTxHandle(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {masterCard("mc-1", masterID, "alpha", "first", 0)},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{byID: map[string]*domain.Cardgroup{destID: mustCardgroup(t, destID, ownerID, "My Deck")}}
	runner, _, _ := recordingTxRunner(t)

	uc := newMasterDeckUsecaseWithTx(cg, card, user, userCG, runner, newTestLogger())

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.NoError(t, err)

	assert.Zero(t, cg.pooledCalls, "the merge must not probe the master on a pooled connection")
	require.Len(t, cg.txHandles, 1, "the merge probes the published master exactly once, on its transaction")
	require.Len(t, user.upsertTxHandles, 1)
	assert.Same(t, user.upsertTxHandles[0], cg.txHandles[0],
		"the published probe and the card upsert must share one transaction handle")
}

// TestMasterDeckUsecase_CopyMasterToUser_PublishedProbeRunsOnTxHandle is the
// copy-path mirror of the merge assertion above: same handle-identity contract,
// exercised through CopyMasterToUser (which SeedForNewUser shares).
func TestMasterDeckUsecase_CopyMasterToUser_PublishedProbeRunsOnTxHandle(t *testing.T) {
	t.Parallel()
	const masterID = "m-txhandle"

	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Deck")}}
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {masterCard("mc1", masterID, "f1", "b1", 0)},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{}

	uc, _, _ := newSeedUsecase(t, cg, card, user, userCG)

	_, err := uc.CopyMasterToUser(context.Background(), masterID, "owner-txhandle")
	require.NoError(t, err)

	assert.Zero(t, cg.pooledCalls, "the copy must not probe the master on a pooled connection")
	require.Len(t, cg.txHandles, 1)
	require.Len(t, user.upsertTxHandles, 1)
	assert.Same(t, user.upsertTxHandles[0], cg.txHandles[0],
		"the published probe and the card upsert must share one transaction handle")
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_DestNotFound_Validation(t *testing.T) {
	t.Parallel()
	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{}, &fakeMasterCardRepo{}, &fakeUserCardRepo{},
		&fakeUserCG{byID: map[string]*domain.Cardgroup{}}, // FindByID → ErrNotFound
		stubTxRunner, newTestLogger(),
	)
	_, err := uc.MergeMasterIntoCardgroup(context.Background(), "master-id", domain.CardgroupID("33333333-3333-7333-8333-333333333333"), domain.UserID("u"))
	var ve *ucerr.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "cardgroupId", ve.Field)
}

// TestMasterDeckUsecase_MergeMasterIntoCardgroup_MasterUnpublishedMidFlight_NoImport
// pins the TOCTOU window on the merge write path. The destination
// ownership gate passes, but the master is no longer in the published set when the
// merge re-reads it (modelling an unpublish that landed after
// MasterCatalogUsecase's FindPublishedByID gate). The published-scoped re-read
// yields repository.ErrNotFound, so no master cards are listed or upserted. This
// covers the state the re-read observes; that the re-read also serialises an
// unpublish arriving later is a property of its FOR SHARE lock, pinned by the
// real-DB tests in the repository package.
func TestMasterDeckUsecase_MergeMasterIntoCardgroup_MasterUnpublishedMidFlight_NoImport(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	// The master has cards, but is ABSENT from the published set (byID) — the state
	// after an unpublish that landed once the caller's gate had already passed.
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {masterCard("mc-1", masterID, "alpha", "first", 0)},
	}}
	user := &fakeUserCardRepo{}
	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{}}, // empty published set → FindPublishedByID → ErrNotFound
		card,
		user,
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrNotFound, "the published-scoped re-read must surface ErrNotFound for an unpublished master")
	assert.Equal(t, 0, user.upsertCall, "no cards are upserted when the master is no longer published")
	assert.Empty(t, user.captured, "no draft content is snapshotted into the destination")
}

// TestMasterDeckUsecase_MergeMasterIntoCardgroup_EmptyDeck_ReturnsNotFound is
// the merge-side mirror of the copy case: a deck that lost its last card between
// the catalog probe and the enumeration must not report a successful 0/0 merge
// against a deck that has left the catalog. It collapses into
// repository.ErrNotFound, which MergeMaster maps to its not-found outcome, and
// the destination is left untouched.
func TestMasterDeckUsecase_MergeMasterIntoCardgroup_EmptyDeck_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id-empty"

	destCG := mustCardgroup(t, destID, ownerID, "My Deck")
	userCard := &fakeUserCardRepo{}

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}}, // the catalog probe still passes (and runs outside the tx)
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{masterID: {}}},                        // ...but the enumeration is empty
		userCard,
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner,
		newTestLogger(),
	)

	res, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrNotFound,
		"an empty master must collapse into the same not-found the catalog uses for an unknown id")
	assert.Nil(t, res)
	assert.Zero(t, userCard.upsertCall, "the destination must not be written for a deck that left the catalog")
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_ListCardsError_PropagatesChain(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{listErr: errors.New("list cards failed")},
		&fakeUserCardRepo{},
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: merge master into cardgroup")
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_UpsertCardsError_PropagatesChain(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	masterCards := []*domain.MasterCard{
		masterCard("mc-1", masterID, "alpha", "first", 0),
	}
	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{masterID: masterCards}},
		&fakeUserCardRepo{upsertErr: errors.New("upsert failed")},
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: merge master into cardgroup")
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_ContextCancelled_PassesThrough(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{listErr: context.Canceled},
		&fakeUserCardRepo{},
		&fakeUserCG{byID: map[string]*domain.Cardgroup{destID: destCG}},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	assertCancelled(t, err)
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_PostTxFindByID_ContextCancelled_PassesThrough(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	// One card so the tx body executes and reaches the post-tx FindByID.
	masterCards := []*domain.MasterCard{
		masterCard("mc-1", masterID, "alpha", "first", 0),
	}
	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{masterID: masterCards}},
		&fakeUserCardRepo{},
		&fakeUserCG{
			byID:              map[string]*domain.Cardgroup{destID: destCG},
			findByIDErr:       context.Canceled,
			findByIDErrOnCall: 2, // call 1 = ownership gate (succeeds), call 2 = post-tx read (fails)
		},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	// The post-tx guard returns the bare context sentinel (no eris wrap).
	if err != context.Canceled {
		t.Fatalf("expected bare context.Canceled sentinel, got: %v (%T)", err, err)
	}
	assertCancelled(t, err)
}

func TestMasterDeckUsecase_MergeMasterIntoCardgroup_PostTxFindByIDError_PropagatesChain(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	// One card so the tx body executes and reaches the post-tx FindByID.
	masterCards := []*domain.MasterCard{
		masterCard("mc-1", masterID, "alpha", "first", 0),
	}
	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{masterID: masterCards}},
		&fakeUserCardRepo{},
		&fakeUserCG{
			byID:              map[string]*domain.Cardgroup{destID: destCG},
			findByIDErr:       errors.New("db down"),
			findByIDErrOnCall: 2, // call 1 = ownership gate (succeeds), call 2 = post-tx read (fails)
		},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	assertInternalChain(t, err, "usecase: master deck: merge master into cardgroup: find destination")
}

// TestMasterDeckUsecase_MergeMasterIntoCardgroup_PostTxDestVanished_NotErrNotFound
// pins the translation of the post-commit destination read-back. The merge has
// already committed when the destination cardgroup is deleted concurrently (no
// FOR UPDATE is held on that row), so FindByID yields repository.ErrNotFound. The
// sentinel must NOT travel up: MergeMaster maps any repository.ErrNotFound from
// this delegate to the master-not-found outcome, which would report the catalog
// deck as missing and hide the committed merge.
func TestMasterDeckUsecase_MergeMasterIntoCardgroup_PostTxDestVanished_NotErrNotFound(t *testing.T) {
	t.Parallel()
	const ownerID = "11111111-1111-7111-8111-111111111111"
	const destID = "22222222-2222-7222-8222-222222222222"
	const masterID = "master-id"

	// One card so the tx body executes and reaches the post-tx FindByID.
	masterCards := []*domain.MasterCard{
		masterCard("mc-1", masterID, "alpha", "first", 0),
	}
	destCG := mustCardgroup(t, destID, ownerID, "My Deck")

	uc := newMasterDeckUsecaseWithTx(
		&fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Master")}},
		&fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{masterID: masterCards}},
		&fakeUserCardRepo{},
		&fakeUserCG{
			byID:              map[string]*domain.Cardgroup{destID: destCG},
			findByIDErr:       repository.ErrNotFound,
			findByIDErrOnCall: 2, // call 1 = ownership gate (succeeds), call 2 = post-tx read (destination deleted)
		},
		stubTxRunner,
		newTestLogger(),
	)

	_, err := uc.MergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID(ownerID))
	require.Error(t, err)
	require.NotErrorIs(t, err, repository.ErrNotFound, "the post-commit read-back must not surface the ErrNotFound sentinel")
	assertInternalChain(t, err, "usecase: master deck: merge master into cardgroup: destination cardgroup vanished after merge commit")
}

// --- PreviewMergeMasterIntoCardgroup tests ------------------------------------

func TestPreviewMergeMasterIntoCardgroup_CountsAddedAndUpdated(t *testing.T) {
	t.Parallel()

	const masterID = "m1"
	const destID = "cg-1"
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {
			masterCard("mc1", masterID, "Apple", "a", 0),
			masterCard("mc2", masterID, "Banana", "b", 1),
			masterCard("mc3", masterID, "Cherry", "c", 2),
		},
	}}
	// Destination already has "Apple" and "Banana" (case-sensitive). "cherry" lower
	// is NOT present, so all three master fronts: 2 updated, 1 added.
	user := &fakeUserCardRepo{existingFronts: map[string]map[string]bool{
		destID: {"Apple": true, "Banana": true},
	}}
	userCG := &fakeUserCG{byID: map[string]*domain.Cardgroup{
		destID: {ID: domain.CardgroupID(destID), OwnerID: "owner-1", Name: domain.CardgroupName("My Deck")},
	}}
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{masterID: masterCG(masterID, "Starter")}}

	uc := newMasterDeckUsecaseWithTx(cg, card, user, userCG, stubTxRunner, newTestLogger())

	got, err := uc.PreviewMergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID("owner-1"))
	require.NoError(t, err)
	require.Equal(t, int64(1), got.Added, "Cherry is new")
	require.Equal(t, int64(2), got.Updated, "Apple + Banana already present")
	assert.Equal(t, 1, cg.pooledCalls, "the dry run verifies published status on a pooled connection")
	assert.Empty(t, cg.txHandles, "a read-only dry run opens no transaction and takes no row lock")
}

// TestPreviewMergeMasterIntoCardgroup_MasterUnpublished_ReturnsNotFound pins the
// preview/merge parity decision: the dry run re-verifies published status the way
// the merge does, so an unpublished deck is refused by both rather than producing
// a tally the follow-up merge then rejects. Master cards outlive an unpublish, so
// without this probe the enumeration below would succeed and report a tally.
func TestPreviewMergeMasterIntoCardgroup_MasterUnpublished_ReturnsNotFound(t *testing.T) {
	t.Parallel()

	const masterID = "m-unpublished"
	const destID = "cg-1"
	card := &fakeMasterCardRepo{byMaster: map[string][]*domain.MasterCard{
		masterID: {masterCard("mc1", masterID, "Apple", "a", 0)},
	}}
	user := &fakeUserCardRepo{}
	userCG := &fakeUserCG{byID: map[string]*domain.Cardgroup{
		destID: {ID: domain.CardgroupID(destID), OwnerID: "owner-1", Name: domain.CardgroupName("My Deck")},
	}}
	// Empty published set: the deck was unpublished after the catalog gate passed.
	cg := &fakeMasterCGRepo{byID: map[string]*domain.MasterCardgroup{}}

	uc := newMasterDeckUsecaseWithTx(cg, card, user, userCG, stubTxRunner, newTestLogger())

	_, err := uc.PreviewMergeMasterIntoCardgroup(context.Background(), masterID, domain.CardgroupID(destID), domain.UserID("owner-1"))
	require.ErrorIs(t, err, repository.ErrNotFound,
		"the dry run must surface ErrNotFound so PreviewMergeMaster collapses it into the not-found outcome")
	assert.Zero(t, user.countFrontsCalls, "no tally is computed for a deck that has left the catalog")
}

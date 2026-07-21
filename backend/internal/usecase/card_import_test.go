package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// mockDictCardRepo captures the cards passed to UpsertManyTx and returns a
// caller-controlled split of inserts vs. updates. The split lets the unit
// test drive exact assertions without a real Postgres backend.
type mockDictCardRepo struct {
	// inserted/updated drive the simulated row-classification split.
	inserted int64
	updated  int64
	// returnErr, when non-nil, is returned from UpsertManyTx unchanged.
	returnErr error
	// preExisting holds the fronts of rows that should classify as updates.
	// When set, the mock derives inserted/updated by inspecting the passed-in
	// cards instead of using the static counts above. This lets tests assert
	// that the updated card carries the new back text.
	preExisting map[string]string
	captured    []*domain.Card
	upsertCalls int
}

func (m *mockDictCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error) {
	m.upsertCalls++
	for _, c := range cards {
		// Defensive deep copy so subsequent caller-side mutation cannot alter
		// what the test inspects.
		clone := *c
		m.captured = append(m.captured, &clone)
	}
	if m.returnErr != nil {
		return repository.UpsertManyTxResult{}, m.returnErr
	}
	if m.preExisting != nil {
		var ins, upd int64
		for _, c := range cards {
			if _, ok := m.preExisting[string(c.Front)]; ok {
				upd++
			} else {
				ins++
			}
		}
		return repository.UpsertManyTxResult{Inserted: ins, Updated: upd}, nil
	}
	return repository.UpsertManyTxResult{Inserted: m.inserted, Updated: m.updated}, nil
}

type mockCardImportCardgroupRepo struct {
	findResult *domain.Cardgroup
	findErr    error
	findCalls  int
}

func (m *mockCardImportCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	m.findCalls++
	return m.findResult, m.findErr
}

func ownedCardImportCardgroupRepo(ownerID string) *mockCardImportCardgroupRepo {
	return &mockCardImportCardgroupRepo{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-target"), OwnerID: domain.UserID(ownerID)},
	}
}

// dictTxRunner returns a txRunner that invokes fn with a non-nil sentinel
// *gorm.DB. The mock repository ignores the value, so an empty &gorm.DB{} is
// sufficient to prove the closure ran inside the helper.
func dictTxRunner() (txRunner, *int) {
	calls := 0
	sentinel := &gorm.DB{}
	return func(_ context.Context, fn func(tx *gorm.DB) error) error {
		calls++
		return fn(sentinel)
	}, &calls
}

// ---------------------------------------------------------------------------
// Payload builders
// ---------------------------------------------------------------------------

// jpRunes returns a Hiragana "a" repeated n times. The lexer accepts Hiragana
// runs as a DEFINITION, so the resulting string is a valid back token. Built
// from rune code points so the test source stays ASCII-only per the
// repository language policy.
func jpRunes(n int) string {
	const hiraganaA rune = 0x3042 // U+3042 HIRAGANA LETTER A
	out := make([]rune, n)
	for i := range out {
		out[i] = hiraganaA
	}
	return string(out)
}

// uniqueBack returns a Hiragana definition that contains a numeric suffix so
// each row's back string is unique. Built from rune code points to keep the
// committed source ASCII.
func uniqueBack(suffix int) string {
	// Use Katakana digits area (U+FF10..U+FF19) for the suffix so the lexer
	// treats every rune as part of the DEFINITION.
	const katakanaA rune = 0x30A2 // U+30A2 KATAKANA LETTER A
	digits := []rune{}
	if suffix == 0 {
		digits = append(digits, 0xFF10)
	} else {
		s := suffix
		for s > 0 {
			digits = append([]rune{rune(0xFF10 + (s % 10))}, digits...)
			s /= 10
		}
	}
	return string(katakanaA) + string(digits)
}

// buildPayload concatenates "front<space>back\n" for each input pair and
// base64-encodes the result. Each (front, back) pair must satisfy the textdic
// grammar (ASCII front, Japanese-script back) for the parser to accept it.
func buildPayload(t *testing.T, pairs [][2]string) string {
	t.Helper()
	var b strings.Builder
	for _, p := range pairs {
		b.WriteString(p[0])
		b.WriteString(" ")
		b.WriteString(p[1])
		b.WriteString("\n")
	}
	return base64.StdEncoding.EncodeToString([]byte(b.String()))
}

func TestCardImportUsecase_ValidateHappyPath(t *testing.T) {
	t.Parallel()

	payload := buildPayload(t, [][2]string{{"apple", jpRunes(3)}})
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())

	out, err := uc.Validate(authedCtx("user-1"), payload)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Valid {
		t.Fatalf("expected Valid=true, got false: %+v", out)
	}
	if len(out.ParsedCards) != 1 {
		t.Fatalf("expected 1 parsed card, got %d", len(out.ParsedCards))
	}
	if out.ParsedCards[0].Front != "apple" || out.ParsedCards[0].Back == "" || out.ParsedCards[0].Line != 1 {
		t.Fatalf("unexpected parsed card: %+v", out.ParsedCards[0])
	}
	if len(out.Errors) != 0 {
		t.Fatalf("expected no validation errors, got %+v", out.Errors)
	}
}

func TestCardImportUsecase_ValidateRequiresAuthenticatedUser(t *testing.T) {
	t.Parallel()

	payload := buildPayload(t, [][2]string{{"apple", jpRunes(3)}})
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())

	_, err := uc.Validate(anonCtx(), payload)

	assertUnauthenticated(t, err)
}

func TestCardImportUsecase_ValidateRequiresNonEmptyCallerSub(t *testing.T) {
	t.Parallel()

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	uc.processCardImport = func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		t.Fatal("processCardImport should not be called for empty caller sub")
		return nil, nil, nil
	}

	_, err := uc.Validate(authedCtx(""), "not-valid-base64")

	assertUnauthenticated(t, err)
}

func TestCardImportUsecase_ValidatePayloadErrors(t *testing.T) {
	t.Parallel()

	t.Run("empty payload", func(t *testing.T) {
		t.Parallel()
		uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
		_, err := uc.Validate(authedCtx("user-1"), "")
		assertValidationError(t, err, "payload", "payload must not be empty")
	})

	t.Run("bad base64", func(t *testing.T) {
		t.Parallel()
		uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
		_, err := uc.Validate(authedCtx("user-1"), "!!!not-base64!!!")
		assertValidationError(t, err, "payload", "payload must be standard base64-encoded text")
	})
}

func TestCardImportUsecase_ValidateReturnsParserDiagnostics(t *testing.T) {
	t.Parallel()

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	payload := base64.StdEncoding.EncodeToString([]byte("orphan"))

	out, err := uc.Validate(authedCtx("user-1"), payload)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected Valid=false for parser diagnostics")
	}
	if len(out.ParsedCards) != 0 {
		t.Fatalf("expected no parsed cards, got %+v", out.ParsedCards)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected 1 validation error, got %+v", out.Errors)
	}
	if out.Errors[0].Kind != CardImportErrKindFrontOnly || out.Errors[0].Snippet != "orphan" {
		t.Fatalf("unexpected validation error: %+v", out.Errors[0])
	}
}

func TestCardImportUsecase_ValidateParserFailure(t *testing.T) {
	t.Parallel()

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	uc.processCardImport = func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return nil, nil, errors.New("parser boom")
	}
	payload := base64.StdEncoding.EncodeToString([]byte("apple " + jpRunes(3)))

	_, err := uc.Validate(authedCtx("user-1"), payload)

	assertInternalChain(t, err, "usecase: card import validate: parse")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCardImportUsecase_OwnerAllInserts covers the happy path where every row
// in the payload is brand new for the target cardgroup. Inserted should equal
// len(payload) and Updated should be zero.
func TestCardImportUsecase_OwnerAllInserts(t *testing.T) {
	t.Parallel()

	const n = 100
	pairs := make([][2]string, n)
	for i := 0; i < n; i++ {
		pairs[i] = [2]string{stringFront("front", i), uniqueBack(i)}
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{inserted: n, updated: 0}
	cgRepo := ownedCardImportCardgroupRepo("user-1")
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != n {
		t.Fatalf("Inserted = %d, want %d", out.Inserted, n)
	}
	if out.Updated != 0 {
		t.Fatalf("Updated = %d, want 0", out.Updated)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", out.Errors)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 tx invocation, got %d", *calls)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call, got %d", repo.upsertCalls)
	}
	if len(repo.captured) != n {
		t.Fatalf("expected %d captured cards, got %d", n, len(repo.captured))
	}
	if cgRepo.findCalls != 1 {
		t.Fatalf("expected 1 cardgroup ownership lookup, got %d", cgRepo.findCalls)
	}
	// Card imports are content-only. Per-user FSRS rows are created lazily on
	// first swipe, outside the import path.
	for i, c := range repo.captured {
		if c.CardgroupID != "cg-target" {
			t.Fatalf("captured[%d] CardgroupID=%q, want cg-target", i, c.CardgroupID)
		}
	}
}

// TestCardImportUsecase_OwnerMixedInsertsAndUpdates covers the mixed case:
// 50 fronts already exist (with old backs), 50 fronts are new. The payload
// supplies new backs for the first 50; the mock repo classifies each card by
// looking up its front in preExisting. The test asserts the (50, 50) split
// AND that the captured "shared" rows carry the *new* back text.
func TestCardImportUsecase_OwnerMixedInsertsAndUpdates(t *testing.T) {
	t.Parallel()

	const n = 50
	pairs := make([][2]string, 0, 2*n)
	preExisting := make(map[string]string, n)
	// 50 shared fronts: each new back is "new-<i>".
	for i := 0; i < n; i++ {
		front := stringFront("shared", i)
		newBack := uniqueBack(i + 1000) // disambiguated from the "fresh" range
		pairs = append(pairs, [2]string{front, newBack})
		preExisting[front] = newBack
	}
	// 50 brand new fronts.
	for i := 0; i < n; i++ {
		front := stringFront("fresh", i)
		newBack := uniqueBack(i)
		pairs = append(pairs, [2]string{front, newBack})
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{preExisting: preExisting}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != n {
		t.Fatalf("Inserted = %d, want %d", out.Inserted, n)
	}
	if out.Updated != n {
		t.Fatalf("Updated = %d, want %d", out.Updated, n)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", out.Errors)
	}

	// Spot-check: pick a shared front and confirm the captured card carries
	// the *new* back from the payload, not the legacy stub from preExisting's
	// "old" state. (Here preExisting only stores the post-update value, but
	// the assertion still locks in that the upsert sent the payload's back
	// to the repository — i.e. the new text, not whatever the DB held.)
	const probeIdx = 7
	wantFront := stringFront("shared", probeIdx)
	wantBack := uniqueBack(probeIdx + 1000)
	var found bool
	for _, c := range repo.captured {
		if string(c.Front) == wantFront {
			if string(c.Back) != wantBack {
				t.Fatalf("captured shared front=%q has Back=%q, want %q", wantFront, c.Back, wantBack)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("captured cards do not contain shared front %q", wantFront)
	}
}

// TestCardImportUsecase_NonOwnerUnauthenticated covers the ownership gate: an
// authenticated user who does not own the target cardgroup is rejected before
// the parser or repo is touched.
func TestCardImportUsecase_NonOwnerUnauthenticated(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	cgRepo := ownedCardImportCardgroupRepo("owner-2")
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertUnauthenticated(t, err)
	if cgRepo.findCalls != 1 {
		t.Fatalf("expected 1 cardgroup ownership lookup, got %d", cgRepo.findCalls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on unauthorized owner, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations on unauthorized owner, got %d", *calls)
	}
}

func TestCardImportUsecase_MissingCardgroupBadInput(t *testing.T) {
	t.Parallel()

	payload := buildPayload(t, [][2]string{{"apple", jpRunes(3)}})
	repo := &mockDictCardRepo{}
	cgRepo := &mockCardImportCardgroupRepo{findErr: repository.ErrNotFound}
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "missing",
		Payload:     payload,
	})

	assertValidationError(t, err, "cardgroupId", "cardgroup not found")
	if cgRepo.findCalls != 1 {
		t.Fatalf("expected 1 cardgroup lookup, got %d", cgRepo.findCalls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls for missing cardgroup, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations for missing cardgroup, got %d", *calls)
	}
}

// TestCardImportUsecase_AnonymousUnauthenticated covers the auth gate: a
// caller with no AuthUser in context is rejected with UNAUTHENTICATED before a
// cardgroup ownership lookup.
func TestCardImportUsecase_AnonymousUnauthenticated(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	cgRepo := ownedCardImportCardgroupRepo("user-1")
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	_, err := uc.Import(anonCtx(), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertUnauthenticated(t, err)
	if cgRepo.findCalls != 0 {
		t.Fatalf("expected 0 cardgroup lookups on anonymous, got %d", cgRepo.findCalls)
	}
}

func TestCardImportUsecase_EmptyCallerSubUnauthenticated(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{}
	cgRepo := ownedCardImportCardgroupRepo("user-1")
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx(""), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     "not-valid-base64",
	})

	assertUnauthenticated(t, err)
	if cgRepo.findCalls != 0 {
		t.Fatalf("expected 0 cardgroup lookups on empty caller sub, got %d", cgRepo.findCalls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on empty caller sub, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations on empty caller sub, got %d", *calls)
	}
}

// TestCardImportUsecase_PayloadOverCapBadInput covers the 5000-row cap: a
// payload that parses to >5000 rows is rejected with BAD_USER_INPUT before
// the repo is touched. Empty backs would surface as parse errors instead, so
// the test uses well-formed rows to drive the parsed-row count past the cap.
func TestCardImportUsecase_PayloadOverCapBadInput(t *testing.T) {
	t.Parallel()

	const n = cardImportParsedRowCap + 1
	pairs := make([][2]string, n)
	for i := 0; i < n; i++ {
		pairs[i] = [2]string{stringFront("front", i), uniqueBack(i)}
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertValidationError(t, err, "payload", "")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on over-cap payload, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations on over-cap payload, got %d", *calls)
	}
}

// TestCardImportUsecase_BadRowsSurfaceAsErrors covers the partial-failure
// shape where lexer-level junk is reported as parse errors, but valid rows
// before and after the junk still reach the repository.
//
// Lexer-level junk (e.g. '@') is recovered inside the lexer: it consumes the
// rest of the malformed line and emits NEWLINE, which the grammar's blank-line
// production `entry: NEWLINE` shifts, so the parser never enters error recovery
// and the well-formed rows on either side survive untouched. Lone-WORD /
// lone-DEFINITION lines are matched by the explicit `entry: WORD` / `entry:
// DEFINITION` skip productions and surface as "FRONT_ONLY" / "BACK_ONLY"
// entries without disturbing the accumulator.
func TestCardImportUsecase_BadRowsSurfaceAsErrors(t *testing.T) {
	t.Parallel()

	// Build a payload with three valid rows interleaved with lexer failures.
	// The parser should report the junk lines and still preserve the valid
	// rows on either side.
	var b strings.Builder
	b.WriteString("apple ")
	b.WriteString(uniqueBack(1))
	b.WriteString("\n")
	b.WriteString("@\n")
	b.WriteString("dog ")
	b.WriteString(uniqueBack(2))
	b.WriteString("\n")
	b.WriteString("@\n")
	b.WriteString("cat ")
	b.WriteString(uniqueBack(3))
	b.WriteString("\n")
	payload := base64.StdEncoding.EncodeToString([]byte(b.String()))

	// All three valid rows should survive the lexer errors.
	repo := &mockDictCardRepo{inserted: 3}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 3 {
		t.Fatalf("Inserted = %d, want 3", out.Inserted)
	}
	if out.Updated != 0 {
		t.Fatalf("Updated = %d, want 0", out.Updated)
	}
	// Exactly two lexer failures must surface as parse errors.
	if len(out.Errors) != 2 {
		t.Fatalf("expected exactly 2 parse errors, got %d: %+v", len(out.Errors), out.Errors)
	}
	for i, e := range out.Errors {
		if e.Front != "" {
			t.Fatalf("lexer-error Errors[%d].Front = %q, want empty", i, e.Front)
		}
		if e.Back != "" {
			t.Fatalf("lexer-error Errors[%d].Back = %q, want empty", i, e.Back)
		}
	}
	// The repo must have been called exactly once with the 3 surviving cards.
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call, got %d", repo.upsertCalls)
	}
	if len(repo.captured) != 3 {
		t.Fatalf("expected 3 captured cards, got %d", len(repo.captured))
	}
}

// TestCardImportUsecase_OverLengthFrontAbortsAsValidationError covers the
// construction-time validation now enforced by domain.NewCard: a row whose
// front exceeds CardTextMax parses cleanly through textdic but cannot be built
// into a Card. The import aborts with a typed BAD_USER_INPUT validation error
// (field "front") BEFORE the repository is touched — the all-or-nothing
// semantics the DB CHECK constraint previously enforced via an opaque tx abort.
func TestCardImportUsecase_OverLengthFrontAbortsAsValidationError(t *testing.T) {
	t.Parallel()

	// One valid row followed by a row whose front is one grapheme over the cap.
	// The 501-char ASCII front lexes as a single WORD token.
	var b strings.Builder
	b.WriteString("apple ")
	b.WriteString(uniqueBack(1))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("a", domain.CardTextMax+1))
	b.WriteString(" ")
	b.WriteString(uniqueBack(2))
	b.WriteString("\n")
	payload := base64.StdEncoding.EncodeToString([]byte(b.String()))

	repo := &mockDictCardRepo{}
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})

	assertValidationError(t, err, "front", "")
	if out.Inserted != 0 || out.Updated != 0 {
		t.Fatalf("expected zero-valued output on abort, got %+v", out)
	}
	// All-or-nothing: the repository and tx runner must never be reached.
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 UpsertManyTx calls on abort, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations on abort, got %d", *calls)
	}
}

// TestCardImportUsecase_ValidSkipValidMixedPayload covers the GraphQL
// importCards path with a payload that interleaves a valid row, a lone
// front (skip), and another valid row. The two valid rows must be persisted
// (Inserted == 2), the lone front surfaces as Errors[0] with Kind ==
// "FRONT_ONLY" and an empty Front field, and no skip-only short-circuit fires.
func TestCardImportUsecase_ValidSkipValidMixedPayload(t *testing.T) {
	t.Parallel()

	// buildPayload writes "front<space>back\n" per pair, matching the lexer's
	// WORD<whitespace>DEFINITION grammar. Build the raw text directly because
	// the middle row "orphan" carries no back.
	var b strings.Builder
	b.WriteString("apple ")
	b.WriteString(uniqueBack(1))
	b.WriteString("\n")
	b.WriteString("orphan\n") // lone front: grammar's WORD skip production fires
	b.WriteString("dog ")
	b.WriteString(uniqueBack(2))
	b.WriteString("\n")
	payload := base64.StdEncoding.EncodeToString([]byte(b.String()))

	repo := &mockDictCardRepo{inserted: 2}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 2 {
		t.Fatalf("Inserted = %d, want 2", out.Inserted)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected exactly 1 parse error, got %d: %+v", len(out.Errors), out.Errors)
	}
	if got := out.Errors[0].Kind; got != CardImportErrKindFrontOnly {
		t.Fatalf("Errors[0].Kind = %q, want %q (lone front must be tagged as FRONT_ONLY)", got, CardImportErrKindFrontOnly)
	}
	if got := out.Errors[0].Snippet; got != "orphan" {
		t.Fatalf("Errors[0].Snippet = %q, want %q (parser must capture the WORD token text)", got, "orphan")
	}
	if out.Errors[0].Front != "" {
		t.Fatalf("Errors[0].Front = %q, want empty (skip errors do not carry dedupe token text)", out.Errors[0].Front)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call (valid rows must still persist), got %d", repo.upsertCalls)
	}
	if len(repo.captured) != 2 {
		t.Fatalf("expected 2 captured cards, got %d", len(repo.captured))
	}
}

// TestCardImportUsecase_SkippedLoneFrontDoesNotReachRepository pins the new
// skip-production behavior: a lone front-only line is reported as a skipped
// validation error and never becomes a card with an empty back.
func TestCardImportUsecase_SkippedLoneFrontDoesNotReachRepository(t *testing.T) {
	t.Parallel()

	payload := base64.StdEncoding.EncodeToString([]byte("existing-front\n"))

	repo := &mockDictCardRepo{}
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 0 {
		t.Fatalf("Inserted = %d, want 0", out.Inserted)
	}
	if out.Updated != 0 {
		t.Fatalf("Updated = %d, want 0", out.Updated)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected exactly 1 parse error, got %d: %+v", len(out.Errors), out.Errors)
	}
	if out.Errors[0].Line != 1 {
		t.Fatalf("Errors[0].Line = %d, want 1", out.Errors[0].Line)
	}
	if out.Errors[0].Message != "skipped: front-only line (no definition)" {
		t.Fatalf("Errors[0].Message = %q, want skipped front-only line", out.Errors[0].Message)
	}
	if got := out.Errors[0].Kind; got != CardImportErrKindFrontOnly {
		t.Fatalf("Errors[0].Kind = %q, want %q (lone front must be tagged as FRONT_ONLY)", got, CardImportErrKindFrontOnly)
	}
	if got := out.Errors[0].Snippet; got != "existing-front" {
		t.Fatalf("Errors[0].Snippet = %q, want %q (parser must capture the WORD token text)", got, "existing-front")
	}
	if out.Errors[0].Front != "" || out.Errors[0].Back != "" {
		t.Fatalf("Errors[0] carried fields: %+v, want empty Front/Back", out.Errors[0])
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls for skipped lone front, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations for skipped lone front, got %d", *calls)
	}
}

// TestCardImportUsecase_CardgroupLookupErrorBecomesInternal verifies that a
// non-context-canceled ownership lookup error maps to INTERNAL and never reaches
// the card repository.
func TestCardImportUsecase_CardgroupLookupErrorBecomesInternal(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	cgRepo := &mockCardImportCardgroupRepo{findErr: errors.New("db died")}
	tx, calls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertInternalChain(t, err, "usecase: authorize cardgroup: find by id")
	if cgRepo.findCalls != 1 {
		t.Fatalf("expected 1 cardgroup lookup, got %d", cgRepo.findCalls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls when ownership lookup fails, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations when ownership lookup fails, got %d", *calls)
	}
}

// TestCardImportUsecase_RepoErrorBecomesInternal verifies that a repository
// error (e.g. database failure) is surfaced as an INTERNAL GraphQL error. The
// usecase must still invoke the repository exactly once before returning.
func TestCardImportUsecase_RepoErrorBecomesInternal(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{"apple", jpRunes(3)},
		{"dog", jpRunes(3)},
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{returnErr: errors.New("db: boom")}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertInternalChain(t, err, "usecase: card import: repo")
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call (repo was invoked), got %d", repo.upsertCalls)
	}
}

// TestCardImportUsecase_EmptyCardgroupIDBadInput verifies that an empty
// cardgroupId is rejected with BAD_USER_INPUT before any repo activity occurs.
func TestCardImportUsecase_EmptyCardgroupIDBadInput(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	cgRepo := ownedCardImportCardgroupRepo("user-1")
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(cgRepo, repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "",
		Payload:     payload,
	})
	assertValidationError(t, err, "cardgroupId", "")
	if cgRepo.findCalls != 0 {
		t.Fatalf("expected 0 cardgroup lookups on empty cardgroupId, got %d", cgRepo.findCalls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on empty cardgroupId, got %d", repo.upsertCalls)
	}
}

// TestCardImportUsecase_EmptyPayloadBadInput verifies that an empty payload
// string is rejected with BAD_USER_INPUT before any repo activity occurs.
func TestCardImportUsecase_EmptyPayloadBadInput(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     "",
	})
	assertValidationError(t, err, "payload", "")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on empty payload, got %d", repo.upsertCalls)
	}
}

// TestCardImportUsecase_BadBase64BadInput verifies that a payload that is not
// valid standard base64 is rejected with BAD_USER_INPUT before any repo
// activity occurs.
func TestCardImportUsecase_BadBase64BadInput(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     "not-valid-base64-!@#$",
	})
	assertValidationError(t, err, "payload", "")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on bad base64, got %d", repo.upsertCalls)
	}
}

// TestCardImportUsecase_DuplicateFrontDeduplicatedAndSurfaced verifies that
// when the payload contains two lines with the same front, the later occurrence
// wins (last-write-wins dedup) and the dropped earlier row surfaces as a
// CardImportError with a "DUPLICATE" message. Exactly one card
// reaches the repository and its Back matches the last occurrence.
//
// Note: this test depends on usecase-level deduplication logic. If that logic
// has not landed yet, the test will fail — that is correct behavior because it
// documents the expected contract.
func TestCardImportUsecase_DuplicateFrontDeduplicatedAndSurfaced(t *testing.T) {
	t.Parallel()

	// Two lines with the same front "apple"; the second occurrence ("rubbish")
	// must win. Backs must be valid Japanese-script tokens for the lexer.
	// U+3042 = HIRAGANA LETTER A (fruit back), U+3052 = HIRAGANA LETTER GE (rubbish back).
	fruitBack := string([]rune{0x3042, 0x3043, 0x3044})   // hiragana run
	rubbishBack := string([]rune{0x3052, 0x3053, 0x3054}) // hiragana run (different)

	raw := "apple " + fruitBack + "\napple " + rubbishBack + "\n"
	payload := base64.StdEncoding.EncodeToString([]byte(raw))

	repo := &mockDictCardRepo{inserted: 1, updated: 0}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 1 {
		t.Fatalf("Inserted = %d, want 1 (dedup keeps last occurrence)", out.Inserted)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected exactly 1 duplicate error, got %d: %+v", len(out.Errors), out.Errors)
	}
	if got := out.Errors[0].Kind; got != CardImportErrKindDuplicate {
		t.Fatalf("Errors[0].Kind = %q, want %q (dropped duplicate must be tagged as DUPLICATE)", got, CardImportErrKindDuplicate)
	}
	if got := out.Errors[0].Front; got != "apple" {
		t.Fatalf("Errors[0].Front = %q, want %q (the duplicate row's front)", got, "apple")
	}
	if got := out.Errors[0].Back; got != fruitBack {
		t.Fatalf("Errors[0].Back = %q, want %q (the dropped row's back)", got, fruitBack)
	}
	// The message names the duplicated front and the back that overrode it (the
	// later, winning occurrence), not the dropped row's own back.
	if msg := out.Errors[0].Message; !strings.Contains(msg, "apple") || !strings.Contains(msg, rubbishBack) {
		t.Fatalf("Errors[0].Message = %q, want it to name front %q and winning back %q", msg, "apple", rubbishBack)
	}
	if msg := out.Errors[0].Message; strings.Contains(msg, fruitBack) {
		t.Fatalf("Errors[0].Message = %q, must not embed the dropped back %q", msg, fruitBack)
	}
	// The dedupe path leaves Snippet empty by contract: the duplicate error
	// carries Front + Back, not a raw snippet.
	if got := out.Errors[0].Snippet; got != "" {
		t.Fatalf("Errors[0].Snippet = %q, want empty (dedupe error does not carry a snippet)", got)
	}
	if len(repo.captured) != 1 {
		t.Fatalf("expected 1 card sent to repo, got %d", len(repo.captured))
	}
	if string(repo.captured[0].Back) != rubbishBack {
		t.Fatalf("expected repo card Back=%q (last occurrence wins), got %q", rubbishBack, repo.captured[0].Back)
	}
}

// TestCardImportUsecase_Import_ContextCancelledDuringUpsert verifies that a
// context.Canceled surfaced by the card repository during UpsertManyTx is
// returned to the caller without wrapping — errors.Is(err, context.Canceled)
// must hold. The isContextDone guard in Import passes the raw error through
// rather than wrapping it with eris.Wrap.
func TestCardImportUsecase_Import_ContextCancelledDuringUpsert(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{returnErr: context.Canceled}
	tx, _ := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected errors.Is(err, context.Canceled)=true, got %T: %v", err, err)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call before cancellation, got %d", repo.upsertCalls)
	}
}

// stringFront returns a deterministic front string of the form "<prefix>-<n>"
// using only ASCII so the lexer treats it as a single WORD token.
func stringFront(prefix string, n int) string {
	return prefix + "-" + strconv.Itoa(n)
}

// TestCardImportUsecase_ValidateDetectsRowCap covers the 5000-row cap in the
// preview path: a payload that parses to >5000 rows is reported as a single
// payload-level HARD error (Line 0), and Valid flips to false even though every
// row parsed cleanly. The over-cap payload is an all-or-nothing reject, so the
// preview echoes an EMPTY ParsedCards slice (mirroring Import's whole-batch
// reject) rather than the full parsed set the import can never persist.
func TestCardImportUsecase_ValidateDetectsRowCap(t *testing.T) {
	t.Parallel()

	const n = cardImportParsedRowCap + 1
	pairs := make([][2]string, n)
	for i := 0; i < n; i++ {
		pairs[i] = [2]string{stringFront("front", i), uniqueBack(i)}
	}
	payload := buildPayload(t, pairs)

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	out, err := uc.Validate(authedCtx("user-1"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected Valid=false for an over-cap payload")
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected exactly 1 cap error, got %d: %+v", len(out.Errors), out.Errors)
	}
	e := out.Errors[0]
	if e.Kind != CardImportErrKindHard || e.Line != 0 {
		t.Fatalf("expected a HARD Line-0 row-cap error, got %+v", e)
	}
	if len(out.ParsedCards) != 0 {
		t.Fatalf("expected an empty ParsedCards on the over-cap reject, got %d", len(out.ParsedCards))
	}
}

// TestCardImportUsecase_ValidateDetectsOverLengthFront covers the per-side
// grapheme cap in the preview path for the FRONT side: a row whose front is one
// grapheme over the cap parses cleanly through textdic but is reported as a
// line-attributed HARD error, and Valid flips to false.
func TestCardImportUsecase_ValidateDetectsOverLengthFront(t *testing.T) {
	t.Parallel()

	// Row 1: valid. Row 2: front is CardTextMax+1 ASCII chars (a single WORD token).
	var b strings.Builder
	b.WriteString("apple ")
	b.WriteString(uniqueBack(1))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("a", domain.CardTextMax+1))
	b.WriteString(" ")
	b.WriteString(uniqueBack(2))
	b.WriteString("\n")
	payload := base64.StdEncoding.EncodeToString([]byte(b.String()))

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	out, err := uc.Validate(authedCtx("user-1"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected Valid=false for an over-length front")
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected exactly 1 cap error, got %d: %+v", len(out.Errors), out.Errors)
	}
	e := out.Errors[0]
	if e.Kind != CardImportErrKindHard || e.Line != 2 {
		t.Fatalf("expected a HARD error attributed to line 2, got %+v", e)
	}
	if !strings.Contains(e.Message, "front") {
		t.Fatalf("expected message to name the front side, got %q", e.Message)
	}
}

// TestCardImportUsecase_ValidateDetectsOverLengthBack mirrors the front test for
// the BACK side: a 501-grapheme Hiragana back lexes as a valid DEFINITION token
// but exceeds CardTextMax and is reported with the correct line.
func TestCardImportUsecase_ValidateDetectsOverLengthBack(t *testing.T) {
	t.Parallel()

	// Row 1: valid. Row 2: back is CardTextMax+1 Hiragana runes (a single DEFINITION token).
	var b strings.Builder
	b.WriteString("apple ")
	b.WriteString(uniqueBack(1))
	b.WriteString("\n")
	b.WriteString("banana ")
	b.WriteString(jpRunes(domain.CardTextMax + 1))
	b.WriteString("\n")
	payload := base64.StdEncoding.EncodeToString([]byte(b.String()))

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	out, err := uc.Validate(authedCtx("user-1"), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected Valid=false for an over-length back")
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected exactly 1 cap error, got %d: %+v", len(out.Errors), out.Errors)
	}
	e := out.Errors[0]
	if e.Kind != CardImportErrKindHard || e.Line != 2 {
		t.Fatalf("expected a HARD error attributed to line 2, got %+v", e)
	}
	if !strings.Contains(e.Message, "back") {
		t.Fatalf("expected message to name the back side, got %q", e.Message)
	}
}

// TestCheckImportCaps_ReturnsValidatedVOs pins the single-scan optimization at
// its source: on the pass path validateImportRows returns the trimmed CardText VOs
// parallel to the input words so Import can build cards via
// domain.NewCardFromValidated without a second grapheme scan. The row-cap
// short-circuit returns a nil VO slice and only the payload-level violation
// (no per-row scan).
func TestValidateImportRows_ReturnsValidatedVOs(t *testing.T) {
	t.Parallel()

	words := []textdic.ParsedWord{
		{Front: "  apple  ", Back: "  " + jpRunes(3) + "  ", Line: 1},
		{Front: "dog", Back: jpRunes(2), Line: 2},
	}
	validated, caps := validateImportRows(words)
	if len(caps) != 0 {
		t.Fatalf("expected no cap violations, got %+v", caps)
	}
	if len(validated) != len(words) {
		t.Fatalf("expected %d validated rows parallel to input, got %d", len(words), len(validated))
	}
	// The VOs must be the trimmed ParseCardText output, ready for reuse by
	// NewCardFromValidated — proving the grapheme scan already happened here.
	if validated[0].front != domain.CardText("apple") {
		t.Fatalf("validated[0].front = %q, want trimmed %q", validated[0].front, "apple")
	}
	if validated[0].back != domain.CardText(jpRunes(3)) {
		t.Fatalf("validated[0].back = %q, want trimmed back VO", validated[0].back)
	}
	if validated[1].front != domain.CardText("dog") {
		t.Fatalf("validated[1].front = %q, want %q", validated[1].front, "dog")
	}

	// Over the row cap: nil VO slice + only the payload-level violation.
	over := make([]textdic.ParsedWord, cardImportParsedRowCap+1)
	for i := range over {
		over[i] = textdic.ParsedWord{Front: stringFront("f", i), Back: jpRunes(2), Line: i + 1}
	}
	vOver, capsOver := validateImportRows(over)
	if vOver != nil {
		t.Fatalf("expected a nil VO slice on the row-cap short-circuit, got len %d", len(vOver))
	}
	if len(capsOver) != 1 || capsOver[0].Field != "payload" || capsOver[0].Line != 0 {
		t.Fatalf("expected a single payload-level row-cap violation, got %+v", capsOver)
	}
}

// TestCardImport_BuildLoopReusesValidatedVOs pins the single grapheme-scan-per-row
// optimization structurally: the Import build loop must construct cards via
// domain.NewCardFromValidated (reusing the CardText VOs validateImportRows already
// parsed) and must NOT call domain.NewCard, which re-runs domain.ParseCardText — a
// second grapheme scan of every row's front and back. A regression to NewCard
// would silently double-scan every import batch with no behavioral difference, so
// only a source-level guard can catch it. Mirrors the static-source regression
// guard pattern used on the frontend.
func TestCardImport_BuildLoopReusesValidatedVOs(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("card_import.go")
	if err != nil {
		t.Fatalf("read card_import.go: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "domain.NewCardFromValidated(") {
		t.Fatal("card_import.go must build import cards via domain.NewCardFromValidated to reuse validateImportRows' VOs (one grapheme scan per row)")
	}
	if strings.Contains(s, "domain.NewCard(") {
		t.Fatal("card_import.go must not call domain.NewCard (re-runs domain.ParseCardText, double-scanning each row); use domain.NewCardFromValidated with the VOs from validateImportRows")
	}
}

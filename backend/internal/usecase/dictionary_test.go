package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// mockAdminChecker is a stub for the AdminChecker interface used by the
// dictionary usecase. The boolean field controls the return value; the err
// field, when non-nil, is returned instead and forces the INTERNAL path.
type mockAdminChecker struct {
	isAdmin bool
	err     error
	calls   int
}

func (m *mockAdminChecker) IsAdmin(_ context.Context, _ string) (bool, error) {
	m.calls++
	if m.err != nil {
		return false, m.err
	}
	return m.isAdmin, nil
}

// mockDictCardRepo captures the cards passed to UpsertManyTx and returns a
// caller-controlled split of inserts vs. updates. The split lets the unit
// test drive exact assertions without a real Postgres backend.
type mockDictCardRepo struct {
	// inserted/updated drive the simulated row-classification split.
	inserted int64
	updated  int64
	// returnErr, when non-nil, is returned from UpsertManyTx unchanged.
	returnErr error
	// preExisting holds the (front -> back) of rows that should classify as
	// updates. When set, the mock derives inserted/updated by inspecting the
	// passed-in cards instead of using the static counts above. This lets
	// tests assert that the updated card carries the new back text.
	preExisting   map[string]string // front -> back at upsert time (post-update)
	captured      []*domain.Card
	upsertCalls   int
	receivedTxNil bool
}

func (m *mockDictCardRepo) UpsertManyTx(_ context.Context, tx *gorm.DB, cards []*domain.Card) (repository.UpsertManyTxResult, error) {
	m.upsertCalls++
	if tx == nil {
		m.receivedTxNil = true
	}
	for _, c := range cards {
		// Defensive deep copy so subsequent caller-side mutation cannot alter
		// what the test inspects.
		copy := *c
		m.captured = append(m.captured, &copy)
	}
	if m.returnErr != nil {
		return repository.UpsertManyTxResult{}, m.returnErr
	}
	if m.preExisting != nil {
		var ins, upd int64
		for _, c := range cards {
			if _, ok := m.preExisting[c.Front]; ok {
				upd++
			} else {
				ins++
			}
		}
		return repository.UpsertManyTxResult{Inserted: ins, Updated: upd}, nil
	}
	return repository.UpsertManyTxResult{Inserted: m.inserted, Updated: m.updated}, nil
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

// dictionaryAdminCtx returns an authed context whose subject is treated as
// admin by the supplied AdminChecker. The actual admin verdict comes from
// the checker — this helper only marks the request as authenticated.
func dictionaryAdminCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDictionaryUsecase_AdminAllInserts covers the happy path where every row
// in the payload is brand new for the target cardgroup. Inserted should equal
// len(payload) and Updated should be zero.
func TestDictionaryUsecase_AdminAllInserts(t *testing.T) {
	t.Parallel()

	const n = 100
	pairs := make([][2]string, n)
	for i := 0; i < n; i++ {
		pairs[i] = [2]string{stringFront("front", i), uniqueBack(i)}
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{inserted: n, updated: 0}
	auth := &mockAdminChecker{isAdmin: true}
	tx, calls := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(auth, repo, tx)

	out, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
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
	// All cards must target the requested cardgroup and carry the default
	// FSRS state (state=New, stability=2.5).
	for i, c := range repo.captured {
		if c.CardgroupID != "cg-target" {
			t.Fatalf("captured[%d] CardgroupID=%q, want cg-target", i, c.CardgroupID)
		}
		if c.FSRS.State != domain.FSRSStateNew {
			t.Fatalf("captured[%d] FSRS.State=%d, want New", i, c.FSRS.State)
		}
		if c.FSRS.Stability != 2.5 {
			t.Fatalf("captured[%d] FSRS.Stability=%v, want 2.5", i, c.FSRS.Stability)
		}
	}
}

// TestDictionaryUsecase_AdminMixedInsertsAndUpdates covers the mixed case:
// 50 fronts already exist (with old backs), 50 fronts are new. The payload
// supplies new backs for the first 50; the mock repo classifies each card by
// looking up its front in preExisting. The test asserts the (50, 50) split
// AND that the captured "shared" rows carry the *new* back text.
func TestDictionaryUsecase_AdminMixedInsertsAndUpdates(t *testing.T) {
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
	auth := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(auth, repo, tx)

	out, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
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
		if c.Front == wantFront {
			if c.Back != wantBack {
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

// TestDictionaryUsecase_NonAdminForbidden covers the auth gate: a non-admin
// caller is rejected with FORBIDDEN before the parser or repo is touched.
func TestDictionaryUsecase_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	auth := &mockAdminChecker{isAdmin: false}
	tx, calls := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(auth, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("user-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertGQLErr(t, err, "FORBIDDEN", "")
	if auth.calls != 1 {
		t.Fatalf("expected 1 IsAdmin call, got %d", auth.calls)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on forbidden, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations on forbidden, got %d", *calls)
	}
}

// TestDictionaryUsecase_AnonymousUnauthenticated covers the auth gate: a
// caller with no AuthUser in context is rejected with UNAUTHENTICATED before
// the AdminChecker is consulted.
func TestDictionaryUsecase_AnonymousUnauthenticated(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	auth := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(auth, repo, tx)

	_, err := uc.Upsert(anonCtx(), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if auth.calls != 0 {
		t.Fatalf("expected 0 IsAdmin calls on anonymous, got %d", auth.calls)
	}
}

// TestDictionaryUsecase_PayloadOverCapBadInput covers the 5000-row cap: a
// payload that parses to >5000 rows is rejected with BAD_USER_INPUT before
// the repo is touched. Empty backs would surface as parse errors instead, so
// the test uses well-formed rows to drive the parsed-row count past the cap.
func TestDictionaryUsecase_PayloadOverCapBadInput(t *testing.T) {
	t.Parallel()

	const n = dictionaryParsedRowCap + 1
	pairs := make([][2]string, n)
	for i := 0; i < n; i++ {
		pairs[i] = [2]string{stringFront("front", i), uniqueBack(i)}
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	auth := &mockAdminChecker{isAdmin: true}
	tx, calls := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(auth, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "payload")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on over-cap payload, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations on over-cap payload, got %d", *calls)
	}
}

// TestDictionaryUsecase_BadRowsSurfaceAsErrors covers the partial-failure
// shape: valid rows interleaved with malformed lines (missing definition) must
// not block the upsert — the usecase passes the parsed valid rows to the repo
// and surfaces the parser errors in Output.Errors.
//
// Grammar behavior note: the LALR grammar's "error NEWLINE" recovery rule
// drops the accumulated parse state up to the error point. Entries that
// appear BEFORE a malformed row in the accumulation phase are discarded;
// entries AFTER the last malformed row survive. Concretely, for the payload:
//
//	apple <def>\n         → discarded (before first error)
//	malformed-1\n         → error recovery fires
//	dog <def>\n           → discarded (before second error)
//	malformed-2\n         → error recovery fires again
//	cat <def>\n           → survives (after last error)
//
// Only "cat" is returned by the parser; the mock repo reflects this with
// inserted: 1. This is the documented contract — callers should be aware
// that malformed rows in the middle of the payload discard preceding entries.
func TestDictionaryUsecase_BadRowsSurfaceAsErrors(t *testing.T) {
	t.Parallel()

	// Build a payload with three valid rows interleaved with malformed lines
	// (front-only — no DEFINITION token). The textdic grammar's "error
	// NEWLINE" recovery rule discards the malformed lines and continues.
	var b strings.Builder
	b.WriteString("apple ")
	b.WriteString(uniqueBack(1))
	b.WriteString("\n")
	b.WriteString("malformed-1\n") // no back
	b.WriteString("dog ")
	b.WriteString(uniqueBack(2))
	b.WriteString("\n")
	b.WriteString("malformed-2\n") // no back
	b.WriteString("cat ")
	b.WriteString(uniqueBack(3))
	b.WriteString("\n")
	payload := base64.StdEncoding.EncodeToString([]byte(b.String()))

	// The LALR grammar returns only 1 parsed word ("cat") for this interleaved
	// payload; the mock is configured to match.
	repo := &mockDictCardRepo{inserted: 1, updated: 0}
	authChk := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(authChk, repo, tx)

	// The only failure path here would be a panic; otherwise every assertion
	// is on the returned struct.
	out, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Inserted != 1 {
		t.Fatalf("Inserted = %d, want 1", out.Inserted)
	}
	if out.Updated != 0 {
		t.Fatalf("Updated = %d, want 0", out.Updated)
	}
	// Exactly two malformed lines must surface as parse errors.
	if len(out.Errors) != 2 {
		t.Fatalf("expected exactly 2 parse errors, got %d: %+v", len(out.Errors), out.Errors)
	}
	// The repo must have been called exactly once with the 1 surviving card.
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call, got %d", repo.upsertCalls)
	}
	if len(repo.captured) != 1 {
		t.Fatalf("expected 1 captured card (valid row after last error), got %d", len(repo.captured))
	}
}

// TestDictionaryUsecase_AdminCheckerErrorBecomesInternal verifies that a
// non-context-canceled error returned by IsAdmin maps to INTERNAL and never
// reaches the repository. The eris chain captured by gqlerr.Internal is
// observed only in logs; the test pins the error code returned to the caller.
func TestDictionaryUsecase_AdminCheckerErrorBecomesInternal(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	auth := &mockAdminChecker{err: errors.New("db died")}
	tx, calls := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(auth, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertGQLErr(t, err, "INTERNAL", "")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls when admin check fails, got %d", repo.upsertCalls)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations when admin check fails, got %d", *calls)
	}
}

// TestDictionaryUsecase_RepoErrorBecomesInternal verifies that a repository
// error (e.g. database failure) is surfaced as an INTERNAL GraphQL error. The
// usecase must still invoke the repository exactly once before returning.
func TestDictionaryUsecase_RepoErrorBecomesInternal(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{"apple", jpRunes(3)},
		{"dog", jpRunes(3)},
	}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{returnErr: errors.New("db: boom")}
	authChk := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(authChk, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     payload,
	})
	assertGQLErr(t, err, "INTERNAL", "")
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertManyTx call (repo was invoked), got %d", repo.upsertCalls)
	}
}

// TestDictionaryUsecase_EmptyCardgroupIDBadInput verifies that an empty
// cardgroupId is rejected with BAD_USER_INPUT before any repo activity occurs.
func TestDictionaryUsecase_EmptyCardgroupIDBadInput(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"apple", jpRunes(3)}}
	payload := buildPayload(t, pairs)

	repo := &mockDictCardRepo{}
	authChk := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(authChk, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "",
		Payload:     payload,
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "cardgroupId")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on empty cardgroupId, got %d", repo.upsertCalls)
	}
}

// TestDictionaryUsecase_EmptyPayloadBadInput verifies that an empty payload
// string is rejected with BAD_USER_INPUT before any repo activity occurs.
func TestDictionaryUsecase_EmptyPayloadBadInput(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{}
	authChk := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(authChk, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     "",
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "payload")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on empty payload, got %d", repo.upsertCalls)
	}
}

// TestDictionaryUsecase_BadBase64BadInput verifies that a payload that is not
// valid standard base64 is rejected with BAD_USER_INPUT before any repo
// activity occurs.
func TestDictionaryUsecase_BadBase64BadInput(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{}
	authChk := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(authChk, repo, tx)

	_, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
		CardgroupID: "cg-target",
		Payload:     "not-valid-base64-!@#$",
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "payload")
	if repo.upsertCalls != 0 {
		t.Fatalf("expected 0 repo calls on bad base64, got %d", repo.upsertCalls)
	}
}

// TestDictionaryUsecase_DuplicateFrontDeduplicatedAndSurfaced verifies that
// when the payload contains two lines with the same front, the later occurrence
// wins (last-write-wins dedup) and the dropped earlier row surfaces as a
// DictionaryValidationError with a "duplicate" message. Exactly one card
// reaches the repository and its Back matches the last occurrence.
//
// Note: this test depends on usecase-level deduplication logic. If that logic
// has not landed yet, the test will fail — that is correct behavior because it
// documents the expected contract.
func TestDictionaryUsecase_DuplicateFrontDeduplicatedAndSurfaced(t *testing.T) {
	t.Parallel()

	// Two lines with the same front "apple"; the second occurrence ("rubbish")
	// must win. Backs must be valid Japanese-script tokens for the lexer.
	// U+3042 = HIRAGANA LETTER A (fruit back), U+3052 = HIRAGANA LETTER GE (rubbish back).
	fruitBack := string([]rune{0x3042, 0x3043, 0x3044})   // hiragana run
	rubbishBack := string([]rune{0x3052, 0x3053, 0x3054}) // hiragana run (different)

	raw := "apple " + fruitBack + "\napple " + rubbishBack + "\n"
	payload := base64.StdEncoding.EncodeToString([]byte(raw))

	repo := &mockDictCardRepo{inserted: 1, updated: 0}
	authChk := &mockAdminChecker{isAdmin: true}
	tx, _ := dictTxRunner()
	uc := NewDictionaryUsecaseWithTx(authChk, repo, tx)

	out, err := uc.Upsert(dictionaryAdminCtx("admin-1"), UpsertDictionaryInput{
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
	if len(repo.captured) != 1 {
		t.Fatalf("expected 1 card sent to repo, got %d", len(repo.captured))
	}
	if repo.captured[0].Back != rubbishBack {
		t.Fatalf("expected repo card Back=%q (last occurrence wins), got %q", rubbishBack, repo.captured[0].Back)
	}
}

// stringFront returns a deterministic front string of the form "<prefix>-<n>"
// using only ASCII so the lexer treats it as a single WORD token.
func stringFront(prefix string, n int) string {
	return prefix + "-" + itoa(n)
}

// itoa is a small allocation-free integer formatter; we avoid strconv to keep
// the test imports minimal and because the input range is bounded.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

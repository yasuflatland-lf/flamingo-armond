package usecase

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/textdic"
)

// overCapPayload is a decoded payload one byte past the shared byte cap, and
// atCapPayload sits exactly on it. Both are plain ASCII, so byte length equals
// character count.
func overCapPayload() string { return strings.Repeat("a", cardImportPayloadByteCap+1) }
func atCapPayload() string   { return strings.Repeat("a", cardImportPayloadByteCap) }

// oversizeMessage is the single message both import paths and the preview emit
// for a payload past the byte cap.
func oversizeMessage() string {
	return fmt.Sprintf("payload exceeds %d bytes", cardImportPayloadByteCap)
}

// oneWordProcess stands in for the textdic parser so the byte-cap tests never
// pay for parsing a megabyte. It fails the test if the pipeline reaches it, when
// the caller passes wantCalled=false.
func oneWordProcess(t *testing.T, wantCalled bool) func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
	t.Helper()
	return func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		if !wantCalled {
			t.Fatal("an over-size payload must be rejected before the parser runs")
		}
		return []textdic.ParsedWord{{Front: "apple", Back: "fruit", Line: 1}}, nil, nil
	}
}

// ---------------------------------------------------------------------------
// Payload byte cap — one channel, shared by both import paths
// ---------------------------------------------------------------------------

// An over-size payload fails through the SAME channel as an over-row payload: a
// top-level *ucerr.ValidationError on "payload". It used to reach the caller as a
// success-shaped zero-card result carrying an embedded HARD error, because the
// byte cap was enforced inside textdic.Process rather than alongside the other
// two import caps.
func TestCardImportUsecase_Import_OverSizePayloadIsTopLevelValidationError(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{}
	tx, txCalls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())
	uc.processCardImport = oneWordProcess(t, false)

	_, err := uc.Import(authedCtx("user-1"), ImportCardsInput{CardgroupID: "cg-target", Payload: b64(overCapPayload())})

	assertValidationError(t, err, "payload", oversizeMessage())
	// All-or-nothing: an over-size payload never opens a tx or reaches the repo.
	require.Zero(t, *txCalls, "over-size payload must not open a transaction")
	require.Zero(t, repo.upsertCalls, "over-size payload must not reach the repository")
}

// The master path reports the identical field and message, which is the whole
// point of hoisting the cap out of the parser: both paths now share one checker.
func TestMasterCard_ImportMasterCards_OverSizePayloadIsTopLevelValidationError(t *testing.T) {
	t.Parallel()

	mc := &mockMasterCardWriteRepo{}
	tx, txCalls := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, oneWordProcess(t, false))

	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64(overCapPayload())})

	assertValidationError(t, err, "payload", oversizeMessage())
	require.Zero(t, *txCalls, "over-size payload must not open a transaction")
	require.Zero(t, mc.upsertCalls, "over-size payload must not reach the repository")
}

// Off-by-one guard: a payload landing exactly on the cap is accepted and imported.
func TestCardImportUsecase_Import_PayloadExactlyAtByteCapIsAccepted(t *testing.T) {
	t.Parallel()

	repo := &mockDictCardRepo{inserted: 1}
	tx, txCalls := dictTxRunner()
	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), repo, tx, newTestLogger())
	uc.processCardImport = oneWordProcess(t, true)

	out, err := uc.Import(authedCtx("user-1"), ImportCardsInput{CardgroupID: "cg-target", Payload: b64(atCapPayload())})

	require.NoError(t, err)
	require.Equal(t, int64(1), out.Inserted)
	require.Equal(t, 1, *txCalls)
}

func TestMasterCard_ImportMasterCards_PayloadExactlyAtByteCapIsAccepted(t *testing.T) {
	t.Parallel()

	mc := &mockMasterCardWriteRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, txCalls := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, oneWordProcess(t, true))

	out, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64(atCapPayload())})

	require.NoError(t, err)
	require.Equal(t, int64(1), out.Inserted)
	require.Equal(t, 1, *txCalls)
}

// The preview keeps its own channel for whole-payload rejects — the call
// succeeds, Valid is false, and the violation is a HARD line-0 entry with no
// preview rows — exactly as the row cap already behaved. Only the commit paths
// changed shape.
func TestCardImportUsecase_Validate_OverSizePayloadIsHardLineZero(t *testing.T) {
	t.Parallel()

	uc := NewCardImportUsecaseWithTx(ownedCardImportCardgroupRepo("user-1"), nil, nil, newTestLogger())
	uc.processCardImport = oneWordProcess(t, false)

	out, err := uc.Validate(authedCtx("user-1"), b64(overCapPayload()))

	require.NoError(t, err)
	require.False(t, out.Valid)
	require.Empty(t, out.ParsedCards, "an over-size payload is never previewed row by row")
	require.Len(t, out.Errors, 1)
	require.Equal(t, 0, out.Errors[0].Line)
	require.Equal(t, CardImportErrKindHard, out.Errors[0].Kind)
	require.Equal(t, oversizeMessage(), out.Errors[0].Message)
}

// ---------------------------------------------------------------------------
// The master build loop consumes validateImportRows' VOs
// ---------------------------------------------------------------------------

// Behavioural half of the single-grapheme-scan contract: the row that reaches
// the repository carries the TRIMMED text, which is only true if the build loop
// uses the CardText VOs validateImportRows parsed. A build loop handed the raw
// parser strings would store them verbatim, because
// domain.NewMasterCardFromValidated deliberately does not re-parse.
func TestMasterCard_ImportMasterCards_BuildsRowsFromValidatedVOs(t *testing.T) {
	t.Parallel()

	process := func(string) ([]textdic.ParsedWord, []textdic.ValidationError, error) {
		return []textdic.ParsedWord{{Front: "  Apple  ", Back: "  fruit  ", Line: 1}}, nil, nil
	}
	mc := &mockMasterCardWriteRepo{upsertResult: repository.UpsertManyTxResult{Inserted: 1}}
	tx, _ := dictTxRunner()
	uc := newMasterCardImportUC(t, mc, tx, true, process)

	_, err := uc.ImportMasterCards(authedCtx("admin1"), ImportMasterCardsInput{MasterCardgroupID: "m1", Payload: b64("ignored")})

	require.NoError(t, err)
	require.Len(t, mc.upsertCaptured, 1)
	require.Equal(t, domain.CardText("Apple"), mc.upsertCaptured[0].Front)
	require.Equal(t, domain.CardText("fruit"), mc.upsertCaptured[0].Back)
}

// Structural half of the same contract. Trimming alone cannot distinguish one
// grapheme scan from two — both constructors trim identically — so the "no second
// scan" property is pinned at the source level: ImportMasterCards must build
// through the VO constructor and must not reach for the string-input one.
func TestMasterCard_ImportMasterCards_UsesTheValidatedConstructor(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("master_card.go")
	require.NoError(t, err)
	const marker = "func (u *masterCardUsecase) ImportMasterCards("
	start := strings.Index(string(src), marker)
	require.GreaterOrEqual(t, start, 0, "ImportMasterCards must exist in master_card.go")
	body := string(src)[start:]
	if end := strings.Index(body[len(marker):], "\nfunc "); end >= 0 {
		body = body[:len(marker)+end]
	}

	require.Contains(t, body, "domain.NewMasterCardFromValidated(",
		"the import build loop must reuse validateImportRows' CardText VOs")
	require.NotContains(t, body, "domain.NewMasterCard(",
		"domain.NewMasterCard re-parses both sides — a second grapheme scan per row")
}

package usecase_test

import (
	"testing"

	"backend/internal/domain"
	"backend/internal/usecase"

	"github.com/stretchr/testify/require"
)

// stubClassifier is a usecase.CEFRClassifier for delegation tests.
type stubClassifier struct {
	level domain.CEFRLevel
	ok    bool
	gotIn string
}

func (s *stubClassifier) Classify(front string) (domain.CEFRLevel, bool) {
	s.gotIn = front
	return s.level, s.ok
}

func TestNewCEFRUsecase_NilClassifierPanics(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t,
		"usecase: CEFRUsecase requires a non-nil classifier",
		func() { usecase.NewCEFRUsecase(nil) })
}

func TestCEFRUsecase_Classify_Delegates(t *testing.T) {
	t.Parallel()
	stub := &stubClassifier{level: domain.CEFRB2, ok: true}
	uc := usecase.NewCEFRUsecase(stub)

	level, ok := uc.Classify("nevertheless")
	require.True(t, ok)
	require.Equal(t, domain.CEFRB2, level)
	require.Equal(t, "nevertheless", stub.gotIn)
}

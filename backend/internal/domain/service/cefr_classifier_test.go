package service

import (
	"testing"

	"backend/internal/domain"

	"github.com/stretchr/testify/require"
)

// fakeWordList is an in-memory domain.CEFRWordList for classifier tests.
type fakeWordList map[string]domain.CEFRLevel

func (f fakeWordList) Lookup(w string) (domain.CEFRLevel, bool) {
	l, ok := f[w]
	return l, ok
}

func TestNewCEFRClassifier_NilWordListPanics(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t,
		"service: CEFRClassifier requires a non-nil word list",
		func() { NewCEFRClassifier(nil) })
}

func TestCEFRClassifier_Classify(t *testing.T) {
	t.Parallel()
	words := fakeWordList{
		"cat":     domain.CEFRA1,
		"run":     domain.CEFRA2,
		"water":   domain.CEFRB1,
		"give up": domain.CEFRB2, // multi-word entry
		"give":    domain.CEFRA1,
		"up":      domain.CEFRA1,
		// Whole-phrase C2 outranks the per-token max (B2), so a spacing miss shows up as B2.
		"artificial intelligence": domain.CEFRC2,
		"artificial":              domain.CEFRB2,
		"intelligence":            domain.CEFRB1,
	}
	c := NewCEFRClassifier(words)

	cases := []struct {
		name      string
		front     string
		wantLevel domain.CEFRLevel
		wantOK    bool
	}{
		{"single token", "cat", domain.CEFRA1, true},
		{"whole-string beats tokens", "give up", domain.CEFRB2, true},
		{"highest token wins", "the cat can run", domain.CEFRA2, true},
		{"punctuation + case normalized", "Water!", domain.CEFRB1, true},
		{"no substring match (cat in cathedral)", "cathedral", domain.CEFRUnknown, false},
		{"unknown front", "zzqq", domain.CEFRUnknown, false},
		{"empty front", "", domain.CEFRUnknown, false},
		{"multi-word key with double space", "artificial  intelligence", domain.CEFRC2, true},
		{"multi-word key with tab", "artificial\tintelligence", domain.CEFRC2, true},
		{"multi-word key with NBSP", "artificial\u00a0intelligence", domain.CEFRC2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotLevel, gotOK := c.Classify(tc.front)
			require.Equal(t, tc.wantOK, gotOK)
			require.Equal(t, tc.wantLevel, gotLevel)
		})
	}
}

package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneralUserCardgroupQuotaReached(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		count int64
		want  bool
	}{
		{name: "none", count: 0, want: false},
		{name: "below limit", count: GeneralUserCardgroupLimit - 1, want: false},
		{name: "at limit", count: GeneralUserCardgroupLimit, want: true},
		{name: "over limit", count: GeneralUserCardgroupLimit + 3, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, GeneralUserCardgroupQuotaReached(tc.count))
		})
	}
}

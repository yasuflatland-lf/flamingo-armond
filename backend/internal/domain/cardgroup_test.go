package domain

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCardgroupShape(t *testing.T) {
	t.Parallel()

	cgType := reflect.TypeOf(Cardgroup{})
	want := map[string]reflect.Type{
		"ID":        reflect.TypeOf(CardgroupID("")),
		"OwnerID":   reflect.TypeOf(UserID("")),
		"Name":      reflect.TypeOf(CardgroupName("")),
		"CreatedAt": reflect.TypeOf(time.Time{}),
		"UpdatedAt": reflect.TypeOf(time.Time{}),
	}

	for name, typ := range want {
		field, ok := cgType.FieldByName(name)
		if !ok {
			t.Fatalf("Cardgroup missing field %s", name)
		}
		if field.Type != typ {
			t.Fatalf("Cardgroup.%s type = %v, want %v", name, field.Type, typ)
		}
		if len(field.Tag) != 0 {
			t.Fatalf("Cardgroup.%s should not have struct tags, got %q", name, field.Tag)
		}
	}
}

func TestCardgroup_Rename(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		initial  CardgroupName
		arg      CardgroupName
		wantErr  error
		wantName CardgroupName
	}{
		{
			name:     "zero CardgroupName returns ErrCardgroupNameRequired and leaves Name unchanged",
			initial:  CardgroupName("original"),
			arg:      CardgroupName(""),
			wantErr:  ErrCardgroupNameRequired,
			wantName: CardgroupName("original"),
		},
		{
			name:     "valid CardgroupName mutates Name and returns nil",
			initial:  CardgroupName("original"),
			arg:      CardgroupName("renamed"),
			wantErr:  nil,
			wantName: CardgroupName("renamed"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cg := &Cardgroup{
				ID:        CardgroupID("cg-id-001"),
				OwnerID:   UserID("owner-001"),
				Name:      tc.initial,
				CreatedAt: fixedTime,
				UpdatedAt: fixedTime,
			}

			err := cg.Rename(tc.arg)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantName, cg.Name)

			// Rename must not mutate ID, OwnerID, CreatedAt, or UpdatedAt.
			require.Equal(t, CardgroupID("cg-id-001"), cg.ID)
			require.Equal(t, UserID("owner-001"), cg.OwnerID)
			require.Equal(t, fixedTime, cg.CreatedAt)
			require.Equal(t, fixedTime, cg.UpdatedAt)
		})
	}
}

func TestCardgroup_IsOwnedBy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		owner  string
		userID string
		want   bool
	}{
		{
			name:   "matching userID returns true",
			owner:  "user-abc",
			userID: "user-abc",
			want:   true,
		},
		{
			name:   "mismatching userID returns false",
			owner:  "user-abc",
			userID: "user-xyz",
			want:   false,
		},
		{
			name:   "empty userID always returns false",
			owner:  "user-abc",
			userID: "",
			want:   false,
		},
		{
			name:   "both owner and userID empty returns false",
			owner:  "",
			userID: "",
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cg := &Cardgroup{OwnerID: UserID(tc.owner)}
			require.Equal(t, tc.want, cg.IsOwnedBy(UserID(tc.userID)))
		})
	}
}

func TestCardgroup_IsOwnedBy_TypedEmptyHandle(t *testing.T) {
	t.Parallel()
	cg := Cardgroup{ID: CardgroupID("cg-1"), OwnerID: UserID("u-1")}
	require.True(t, cg.IsOwnedBy(UserID("u-1")))
	require.False(t, cg.IsOwnedBy(UserID("u-2")))
	require.False(t, cg.IsOwnedBy(UserID("")), "empty UserID never matches")
}

func TestNewCardgroup(t *testing.T) {
	t.Parallel()

	owner := UserID("owner-001")
	name := CardgroupName("My Deck")
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	cg, err := NewCardgroup(owner, name, now)

	require.NoError(t, err)
	require.NotNil(t, cg)

	// A fresh UUID v7 id is generated and is non-empty.
	require.NotEmpty(t, cg.ID)

	// OwnerID and Name are carried through verbatim.
	require.Equal(t, owner, cg.OwnerID)
	require.Equal(t, name, cg.Name)

	require.Equal(t, now, cg.CreatedAt)
	require.Equal(t, now, cg.UpdatedAt)

	// Two constructions produce distinct ids.
	other, err := NewCardgroup(owner, name, now)
	require.NoError(t, err)
	require.NotEqual(t, cg.ID, other.ID)
}

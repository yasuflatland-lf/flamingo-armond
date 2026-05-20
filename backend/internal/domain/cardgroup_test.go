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
		"ID":        reflect.TypeOf(""),
		"OwnerID":   reflect.TypeOf(""),
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cg := &Cardgroup{OwnerID: tc.owner}
			require.Equal(t, tc.want, cg.IsOwnedBy(tc.userID))
		})
	}
}

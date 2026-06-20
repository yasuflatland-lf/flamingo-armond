package domain

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleShape(t *testing.T) {
	t.Parallel()

	roleType := reflect.TypeFor[Role]()
	want := map[string]reflect.Type{
		"ID":   reflect.TypeFor[string](),
		"Name": reflect.TypeFor[RoleName](),
	}

	for name, typ := range want {
		field, ok := roleType.FieldByName(name)
		if !ok {
			t.Fatalf("Role missing field %s", name)
		}
		if field.Type != typ {
			t.Fatalf("Role.%s type = %v, want %v", name, field.Type, typ)
		}
		if len(field.Tag) != 0 {
			t.Fatalf("Role.%s should not have struct tags, got %q", name, field.Tag)
		}
	}
}

func TestRoleIsSystem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		role Role
		want bool
	}{
		{name: "admin", role: Role{Name: AdminRoleName}, want: true},
		{name: "general", role: Role{Name: GeneralRoleName}, want: true},
		{name: "empty", role: Role{Name: ""}, want: false},
		{name: "unknown", role: Role{Name: "unknown"}, want: false},
		{name: "case-sensitive Admin", role: Role{Name: "Admin"}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.role.IsSystem())
		})
	}
}

func TestIsLastAdmin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		adminCount int64
		want       bool
	}{
		{name: "no admins", adminCount: 0, want: true},
		{name: "single admin", adminCount: 1, want: true},
		{name: "two admins", adminCount: 2, want: false},
		{name: "negative count clamps to last", adminCount: -1, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, IsLastAdmin(tc.adminCount))
		})
	}
}

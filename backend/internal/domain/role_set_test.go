package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleSet_ContainsAdmin(t *testing.T) {
	t.Parallel()

	admin := Role{ID: "r-admin", Name: AdminRoleName}
	general := Role{ID: "r-general", Name: GeneralRoleName}

	require.True(t, RoleSet{admin}.ContainsAdmin(), "single admin role")
	require.True(t, RoleSet{general, admin}.ContainsAdmin(), "admin among others")
	require.False(t, RoleSet{general}.ContainsAdmin(), "no admin role")
	require.False(t, RoleSet{}.ContainsAdmin(), "empty set has no admin")
	require.False(t, RoleSet(nil).ContainsAdmin(), "nil set has no admin")
}

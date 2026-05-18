package domain

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserShape(t *testing.T) {
	t.Parallel()

	userType := reflect.TypeOf(User{})
	want := map[string]reflect.Type{
		"ID":          reflect.TypeOf(""),
		"DisplayName": reflect.TypeOf((*string)(nil)),
		"Bio":         reflect.TypeOf((*string)(nil)),
		"AvatarURL":   reflect.TypeOf((*string)(nil)),
		"CreatedAt":   reflect.TypeOf(time.Time{}),
		"UpdatedAt":   reflect.TypeOf(time.Time{}),
	}

	for name, typ := range want {
		field, ok := userType.FieldByName(name)
		if !ok {
			t.Fatalf("User missing field %s", name)
		}
		if field.Type != typ {
			t.Fatalf("User.%s type = %v, want %v", name, field.Type, typ)
		}
		if len(field.Tag) != 0 {
			t.Fatalf("User.%s should not have struct tags, got %q", name, field.Tag)
		}
	}
}

func TestUserUpdateProfile(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	t.Run("sets display name and bio", func(t *testing.T) {
		t.Parallel()
		u := User{ID: "u1"}
		dn, err := ParseDisplayName("Alice")
		require.NoError(t, err)
		s := "hello"
		bio, err := ParseBio(&s)
		require.NoError(t, err)

		u.UpdateProfile(dn, bio)

		require.NotNil(t, u.DisplayName)
		require.Equal(t, "Alice", *u.DisplayName)
		require.NotNil(t, u.Bio)
		require.Equal(t, "hello", *u.Bio)
	})

	t.Run("bio nil leaves existing bio untouched", func(t *testing.T) {
		t.Parallel()
		u := User{ID: "u1", Bio: ptr("existing")}
		dn, err := ParseDisplayName("Bob")
		require.NoError(t, err)
		bio, err := ParseBio(nil)
		require.NoError(t, err)

		u.UpdateProfile(dn, bio)

		require.Equal(t, "Bob", *u.DisplayName)
		require.Equal(t, "existing", *u.Bio)
	})

	t.Run("bio explicit empty clears bio", func(t *testing.T) {
		t.Parallel()
		u := User{ID: "u1", Bio: ptr("existing")}
		dn, err := ParseDisplayName("Carol")
		require.NoError(t, err)
		s := ""
		bio, err := ParseBio(&s)
		require.NoError(t, err)

		u.UpdateProfile(dn, bio)

		require.NotNil(t, u.Bio)
		require.Equal(t, "", *u.Bio)
	})
}

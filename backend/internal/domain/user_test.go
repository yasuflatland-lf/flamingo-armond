package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestUserShape(t *testing.T) {
	t.Parallel()

	userType := reflect.TypeOf(User{})
	want := map[string]reflect.Type{
		"ID":          reflect.TypeOf(""),
		"DisplayName": reflect.TypeOf((*DisplayName)(nil)),
		"Bio":         reflect.TypeOf(Bio{}),
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

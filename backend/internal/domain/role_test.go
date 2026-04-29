package domain

import (
	"reflect"
	"testing"
)

func TestRoleShape(t *testing.T) {
	t.Parallel()

	roleType := reflect.TypeOf(Role{})
	want := map[string]reflect.Type{
		"ID":   reflect.TypeOf(""),
		"Name": reflect.TypeOf(""),
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

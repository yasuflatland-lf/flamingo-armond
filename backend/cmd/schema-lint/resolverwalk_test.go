package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestResolverWalkFiles_PerFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		file         string
		wantMappings []ResolverMapping
		wantErr      bool
	}{
		{
			name: "single usecase call extracted correctly",
			file: filepath.Join("testdata", "resolverwalk", "one-call.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:  "updateRole",
					ResolverMethod: "UpdateRole",
					UsecaseCalls: []UsecaseCall{
						{Selector: "AdminRoleUC", Method: "Update"},
					},
				},
			},
		},
		{
			name: "method with no usecase call yields empty UsecaseCalls",
			file: filepath.Join("testdata", "resolverwalk", "no-call.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:  "deleteRole",
					ResolverMethod: "DeleteRole",
					UsecaseCalls:   nil,
				},
			},
		},
		{
			name: "method with multiple calls captures every call in source order",
			file: filepath.Join("testdata", "resolverwalk", "multi-call.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:  "handleSwipe",
					ResolverMethod: "HandleSwipe",
					UsecaseCalls: []UsecaseCall{
						{Selector: "CardgroupUC", Method: "Get"},
						{Selector: "CardUC", Method: "Create"},
					},
				},
			},
		},
		{
			name:    "non-existent file returns error",
			file:    filepath.Join("testdata", "resolverwalk", "does-not-exist.go"),
			wantErr: true,
		},
		// Test 5 (negative case): chains that are NOT three segments must NOT
		// appear in UsecaseCalls. The fixture has two-segment and four-segment
		// call chains; neither matches the r.<Selector>.<Method>() form.
		{
			name: "non-three-segment chains produce empty UsecaseCalls",
			file: filepath.Join("testdata", "resolverwalk", "non-three-segment.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:  "fakeMutation",
					ResolverMethod: "FakeMutation",
					UsecaseCalls:   nil,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolverWalkFiles([]string{tc.file})
			if tc.wantErr {
				if err == nil {
					t.Fatal("ResolverWalkFiles: expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolverWalkFiles: unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.wantMappings, got.Mappings); diff != "" {
				t.Errorf("ResolverWalkFiles Mappings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolverWalkFiles(t *testing.T) {
	t.Parallel()

	got, err := ResolverWalkFiles([]string{
		filepath.Join("testdata", "resolverwalk", "one-call.go"),
		filepath.Join("testdata", "resolverwalk", "no-call.go"),
	})
	if err != nil {
		t.Fatalf("ResolverWalkFiles: unexpected error: %v", err)
	}

	want := []ResolverMapping{
		{
			MutationField:  "updateRole",
			ResolverMethod: "UpdateRole",
			UsecaseCalls: []UsecaseCall{
				{Selector: "AdminRoleUC", Method: "Update"},
			},
		},
		{
			MutationField:  "deleteRole",
			ResolverMethod: "DeleteRole",
			UsecaseCalls:   nil,
		},
	}
	if diff := cmp.Diff(want, got.Mappings); diff != "" {
		t.Errorf("ResolverWalkFiles Mappings mismatch (-want +got):\n%s", diff)
	}
}

func TestToCamelCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"UpdateRole", "updateRole"},
		{"HandleSwipe", "handleSwipe"},
		{"CreateCard", "createCard"},
		{"AdminEditUser", "adminEditUser"},
		{"SetLastViewedCardgroup", "setLastViewedCardgroup"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := toCamelCase(tc.input)
			if got != tc.want {
				t.Errorf("toCamelCase(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

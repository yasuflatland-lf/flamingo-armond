package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestResolverWalk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		file           string
		wantMappings   []ResolverMapping
		wantAdditional map[string][]struct{ Selector, Method string }
		wantErr        bool
	}{
		{
			name: "single usecase call extracted correctly",
			file: filepath.Join("testdata", "resolverwalk", "one-call.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:   "updateRole",
					ResolverMethod:  "UpdateRole",
					UsecaseSelector: "AdminRoleUC",
					UsecaseMethod:   "Update",
				},
			},
			wantAdditional: map[string][]struct{ Selector, Method string }{},
		},
		{
			name: "method with no usecase call yields empty selector and method",
			file: filepath.Join("testdata", "resolverwalk", "no-call.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:   "deleteRole",
					ResolverMethod:  "DeleteRole",
					UsecaseSelector: "",
					UsecaseMethod:   "",
				},
			},
			wantAdditional: map[string][]struct{ Selector, Method string }{},
		},
		{
			name: "method with multiple calls puts first in mapping and rest in AdditionalCalls",
			file: filepath.Join("testdata", "resolverwalk", "multi-call.go"),
			wantMappings: []ResolverMapping{
				{
					MutationField:   "handleSwipe",
					ResolverMethod:  "HandleSwipe",
					UsecaseSelector: "CardgroupUC",
					UsecaseMethod:   "Get",
				},
			},
			wantAdditional: map[string][]struct{ Selector, Method string }{
				"HandleSwipe": {
					{Selector: "CardUC", Method: "Create"},
				},
			},
		},
		{
			name:    "non-existent file returns error",
			file:    filepath.Join("testdata", "resolverwalk", "does-not-exist.go"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolverWalk(tc.file)
			if tc.wantErr {
				if err == nil {
					t.Fatal("ResolverWalk: expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolverWalk: unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.wantMappings, got.Mappings); diff != "" {
				t.Errorf("ResolverWalk Mappings mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantAdditional, got.AdditionalCalls); diff != "" {
				t.Errorf("ResolverWalk AdditionalCalls mismatch (-want +got):\n%s", diff)
			}
		})
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
		{"AdminUpdateUser", "adminUpdateUser"},
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

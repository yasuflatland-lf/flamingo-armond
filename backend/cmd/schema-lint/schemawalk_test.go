package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSchemaWalk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		files       []string
		want        []MutationReturn
		wantErr     bool
		wantErrSubs string
	}{
		{
			name:  "bare object returns IsBare true",
			files: []string{filepath.Join("testdata", "schemawalk", "bare-object.graphql")},
			want: []MutationReturn{
				{Field: "updateRole", ReturnType: "Role!", IsBare: true},
			},
		},
		{
			name:  "union return sets IsBare false",
			files: []string{filepath.Join("testdata", "schemawalk", "union-return.graphql")},
			want: []MutationReturn{
				{Field: "createCard", ReturnType: "CreateCardResult!", IsBare: false},
			},
		},
		{
			name:  "list return sets IsBare false",
			files: []string{filepath.Join("testdata", "schemawalk", "list-return.graphql")},
			want: []MutationReturn{
				{Field: "bulkCreateCards", ReturnType: "[Card!]!", IsBare: false},
			},
		},
		{
			name:  "scalar Boolean return sets IsBare false",
			files: []string{filepath.Join("testdata", "schemawalk", "scalar-return.graphql")},
			want: []MutationReturn{
				{Field: "deleteRole", ReturnType: "Boolean!", IsBare: false},
			},
		},
		{
			name:  "extend type Mutation merges fields with base Mutation",
			files: []string{filepath.Join("testdata", "schemawalk", "extend-mutation.graphql")},
			want: []MutationReturn{
				{Field: "createRole", ReturnType: "Role!", IsBare: true},
				{Field: "updateRole", ReturnType: "UpdateRoleResult!", IsBare: false},
			},
		},
		{
			name:    "non-existent file returns error",
			files:   []string{filepath.Join("testdata", "schemawalk", "does-not-exist.graphql")},
			wantErr: true,
		},
		{
			name:        "schema with no Mutation type returns error",
			files:       []string{filepath.Join("testdata", "schemawalk", "no-mutation.graphql")},
			wantErr:     true,
			wantErrSubs: "no Mutation type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := SchemaWalk(tc.files)
			if tc.wantErr {
				if err == nil {
					t.Fatal("SchemaWalk: expected error, got nil")
				}
				if tc.wantErrSubs != "" && !strings.Contains(err.Error(), tc.wantErrSubs) {
					t.Errorf("SchemaWalk: error %q does not contain %q", err.Error(), tc.wantErrSubs)
				}
				return
			}
			if err != nil {
				t.Fatalf("SchemaWalk: unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SchemaWalk mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

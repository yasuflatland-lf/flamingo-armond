package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestUsecaseWalk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dir     string
		want    []UsecaseMethod
		wantErr bool
	}{
		{
			name: "detects ucerr.NewValidationError",
			dir:  filepath.Join("testdata", "usecasewalk"),
			// We test the whole directory together; sub-tests below isolate each file.
		},
	}
	_ = tests // directory-level test is covered by file-level sub-tests below.

	// File-level sub-tests: parse a single-file directory equivalent by
	// pointing UsecaseWalk at the testdata/usecasewalk directory and
	// filtering results by filename.
	dir := filepath.Join("testdata", "usecasewalk")
	all, err := UsecaseWalk(dir)
	if err != nil {
		t.Fatalf("UsecaseWalk(%q): unexpected error: %v", dir, err)
	}

	byFile := make(map[string][]UsecaseMethod)
	for _, m := range all {
		byFile[m.File] = append(byFile[m.File], m)
	}

	t.Run("emits-validation.go", func(t *testing.T) {
		t.Parallel()
		want := []UsecaseMethod{
			{
				File:            "emits-validation.go",
				ReceiverType:    "*cardUsecase",
				Method:          "Create",
				EmitsTypedError: true,
			},
		}
		got := byFile["emits-validation.go"]
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b UsecaseMethod) bool {
			return a.Method < b.Method
		})); diff != "" {
			t.Errorf("emits-validation.go mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("emits-forbidden.go", func(t *testing.T) {
		t.Parallel()
		want := []UsecaseMethod{
			{
				File:            "emits-forbidden.go",
				ReceiverType:    "*adminRoleUsecase",
				Method:          "Delete",
				EmitsTypedError: true,
			},
		}
		got := byFile["emits-forbidden.go"]
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b UsecaseMethod) bool {
			return a.Method < b.Method
		})); diff != "" {
			t.Errorf("emits-forbidden.go mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("emits-unauth.go", func(t *testing.T) {
		t.Parallel()
		want := []UsecaseMethod{
			{
				File:            "emits-unauth.go",
				ReceiverType:    "*userUsecase",
				Method:          "Me",
				EmitsTypedError: true,
			},
		}
		got := byFile["emits-unauth.go"]
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b UsecaseMethod) bool {
			return a.Method < b.Method
		})); diff != "" {
			t.Errorf("emits-unauth.go mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("no-emit.go", func(t *testing.T) {
		t.Parallel()
		want := []UsecaseMethod{
			{
				File:            "no-emit.go",
				ReceiverType:    "*learnUsecase",
				Method:          "NextDueCards",
				EmitsTypedError: false,
			},
		}
		got := byFile["no-emit.go"]
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b UsecaseMethod) bool {
			return a.Method < b.Method
		})); diff != "" {
			t.Errorf("no-emit.go mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestUsecaseWalkNonExistentDir(t *testing.T) {
	t.Parallel()
	_, err := UsecaseWalk(filepath.Join("testdata", "usecasewalk", "does-not-exist"))
	if err == nil {
		t.Fatal("UsecaseWalk: expected error for non-existent directory, got nil")
	}
}

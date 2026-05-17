package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestUsecaseWalk(t *testing.T) {
	t.Parallel()

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

	// Test 4 (negative cases): false-positive avoidance

	// no-emit-unrelated-ucerr-call.go: method references ucerr.Classify (an
	// exported ucerr symbol that is NOT one of the three matched names). The
	// matcher must not over-fire on any ucerr.X reference.
	t.Run("no-emit-unrelated-ucerr-call.go", func(t *testing.T) {
		t.Parallel()
		got := byFile["no-emit-unrelated-ucerr-call.go"]
		if len(got) == 0 {
			t.Fatal("expected at least one UsecaseMethod for no-emit-unrelated-ucerr-call.go, got none")
		}
		for _, m := range got {
			if m.EmitsTypedError {
				t.Errorf("no-emit-unrelated-ucerr-call.go: method %s: EmitsTypedError = true, want false (matcher must not fire on ucerr.Classify)", m.Method)
			}
		}
	})

	// no-emit-foreign-package-newvalidation.go: method body calls
	// foreign.NewValidationError where the package alias is NOT ucerr. The
	// matcher must pin on the "ucerr" package qualifier specifically.
	t.Run("no-emit-foreign-package-newvalidation.go", func(t *testing.T) {
		t.Parallel()
		got := byFile["no-emit-foreign-package-newvalidation.go"]
		if len(got) == 0 {
			t.Fatal("expected at least one UsecaseMethod for no-emit-foreign-package-newvalidation.go, got none")
		}
		for _, m := range got {
			if m.EmitsTypedError {
				t.Errorf("no-emit-foreign-package-newvalidation.go: method %s: EmitsTypedError = true, want false (matcher must pin on ucerr qualifier)", m.Method)
			}
		}
	})
}

func TestUsecaseWalkNonExistentDir(t *testing.T) {
	t.Parallel()

	t.Run("relative path under testdata", func(t *testing.T) {
		t.Parallel()
		_, err := UsecaseWalk(filepath.Join("testdata", "usecasewalk", "does-not-exist"))
		if err == nil {
			t.Fatal("UsecaseWalk: expected error for non-existent directory, got nil")
		}
		if !strings.Contains(err.Error(), "usecasewalk: usecase dir") {
			t.Errorf("UsecaseWalk: error %q does not contain expected layer prefix", err.Error())
		}
	})

	t.Run("bare nonexistent name", func(t *testing.T) {
		t.Parallel()
		_, err := UsecaseWalk("nonexistent")
		if err == nil {
			t.Fatal("UsecaseWalk: expected error for non-existent directory, got nil")
		}
		if !strings.Contains(err.Error(), "usecasewalk: usecase dir") {
			t.Errorf("UsecaseWalk: error %q does not contain expected layer prefix", err.Error())
		}
	})
}

func TestUsecaseWalkEmptyDir(t *testing.T) {
	t.Parallel()

	// A directory that exists but has no .go files should also error.
	emptyDir := t.TempDir()
	_, err := UsecaseWalk(emptyDir)
	if err == nil {
		t.Fatal("UsecaseWalk: expected error for empty directory, got nil")
	}
	if !strings.Contains(err.Error(), "no Go packages found") {
		t.Errorf("UsecaseWalk: error %q does not contain expected substring", err.Error())
	}
}

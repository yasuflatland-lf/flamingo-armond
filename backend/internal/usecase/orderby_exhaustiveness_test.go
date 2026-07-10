package usecase

import (
	"testing"

	"backend/internal/repository"
)

// The tests in this file guard the three-layer orderBy enum translation:
// usecase.<Aggregate>OrderBy (schema-mirrored strings) -> repository.<Aggregate>OrderBy
// (snake_case column names) via the resolve*OrderBy translators. Each table
// enumerates every usecase enum value and asserts it maps to the expected
// non-empty repository value with no error. A value that falls through to a
// translator's default arm returns the empty repository value plus a
// BAD_USER_INPUT validation error, tripping the guard.
//
// When a new enum value is added to one of these aggregates, add a row here
// alongside the const. If the matching resolve*OrderBy switch arm is missing,
// the new row resolves through the default arm and the test fails — catching
// the silent default->BAD_USER_INPUT drift before it ships.

func TestResolveOrderBy_AllCardOrderByValuesMapped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   CardOrderBy
		want repository.CardOrderBy
	}{
		{CardOrderByID, repository.CardOrderByID},
		{CardOrderByCreatedAt, repository.CardOrderByCreatedAt},
		{CardOrderByUpdatedAt, repository.CardOrderByUpdatedAt},
		{CardOrderByDue, repository.CardOrderByDue},
	}
	for _, c := range cases {
		ob := c.in
		got, _, err := resolveCardOrderBy(&ob, nil)
		if err != nil {
			t.Errorf("resolveCardOrderBy(%q): unexpected error %v", c.in, err)
		}
		if got == "" {
			t.Errorf("resolveCardOrderBy(%q): mapped to the empty repository value (default arm)", c.in)
		}
		if got != c.want {
			t.Errorf("resolveCardOrderBy(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveCardgroupOrderBy_AllValuesMapped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   CardgroupOrderBy
		want repository.CardgroupOrderBy
	}{
		{CardgroupOrderByID, repository.CardgroupOrderByID},
		{CardgroupOrderByCreatedAt, repository.CardgroupOrderByCreatedAt},
		{CardgroupOrderByUpdatedAt, repository.CardgroupOrderByUpdatedAt},
		{CardgroupOrderByName, repository.CardgroupOrderByName},
	}
	for _, c := range cases {
		ob := c.in
		got, _, err := resolveCardgroupOrderBy(&ob, nil)
		if err != nil {
			t.Errorf("resolveCardgroupOrderBy(%q): unexpected error %v", c.in, err)
		}
		if got == "" {
			t.Errorf("resolveCardgroupOrderBy(%q): mapped to the empty repository value (default arm)", c.in)
		}
		if got != c.want {
			t.Errorf("resolveCardgroupOrderBy(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveMasterCatalogOrderBy_AllValuesMapped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   MasterCatalogOrderBy
		want repository.MasterCatalogOrderBy
	}{
		{MasterCatalogOrderBySortOrder, repository.MasterCatalogOrderBySortOrder},
		{MasterCatalogOrderByCreatedAt, repository.MasterCatalogOrderByCreatedAt},
		{MasterCatalogOrderByName, repository.MasterCatalogOrderByName},
	}
	for _, c := range cases {
		ob := c.in
		got, _, err := resolveMasterCatalogOrderBy(&ob, nil)
		if err != nil {
			t.Errorf("resolveMasterCatalogOrderBy(%q): unexpected error %v", c.in, err)
		}
		if got == "" {
			t.Errorf("resolveMasterCatalogOrderBy(%q): mapped to the empty repository value (default arm)", c.in)
		}
		if got != c.want {
			t.Errorf("resolveMasterCatalogOrderBy(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

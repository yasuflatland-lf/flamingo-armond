package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

// callCustomAccessTokenHook invokes public.custom_access_token_hook(event)
// against the supplied connection and returns the resulting JSON decoded
// into a map. The test fails fatally on any SQL or JSON error.
func callCustomAccessTokenHook(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID string, claims map[string]any) map[string]any {
	t.Helper()

	event := map[string]any{
		"user_id": userID,
		"claims":  claims,
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var resultJSON []byte
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT public.custom_access_token_hook($1::jsonb)`, eventJSON).Scan(&resultJSON); err != nil {
		t.Fatalf("query custom_access_token_hook: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return result
}

// claimsFromHookResult extracts the "claims" object from the hook result and
// fails the test if it is missing or the wrong shape.
func claimsFromHookResult(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	rawClaims, ok := result["claims"]
	if !ok {
		t.Fatalf("hook result missing claims: %v", result)
	}
	claims, ok := rawClaims.(map[string]any)
	if !ok {
		t.Fatalf("claims is not an object: %T", rawClaims)
	}
	return claims
}

// appMetadataFromClaims extracts claims.app_metadata. Returns an empty map and
// no error if the key is absent, so callers can probe for missing keys with
// the same code path as present-but-empty.
func appMetadataFromClaims(t *testing.T, claims map[string]any) map[string]any {
	t.Helper()
	raw, ok := claims["app_metadata"]
	if !ok {
		return map[string]any{}
	}
	meta, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("app_metadata is not an object: %T", raw)
	}
	return meta
}

// TestCustomAccessTokenHook_AdminUserGetsRoleClaim verifies that an admin user
// receives app_metadata.role = "admin" in the returned claims.
func TestCustomAccessTokenHook_AdminUserGetsRoleClaim(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	t.Cleanup(func() { db.Close() })

	userID := insertAuthUserForAdmin(t, ctx, db)
	sqlDB := sqlDBForTest(t, db)

	var adminRoleID string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT id FROM public.roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatalf("query admin role id: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_roles (user_id, role_id) VALUES ($1, $2)`, userID, adminRoleID); err != nil {
		t.Fatalf("insert user_roles: %v", err)
	}

	result := callCustomAccessTokenHook(t, ctx, sqlDB, userID, map[string]any{
		"app_metadata": map[string]any{},
	})
	claims := claimsFromHookResult(t, result)
	meta := appMetadataFromClaims(t, claims)

	role, ok := meta["role"].(string)
	if !ok {
		t.Fatalf("app_metadata.role missing or not a string: %v", meta)
	}
	if role != "admin" {
		t.Fatalf("app_metadata.role: got %q, want %q", role, "admin")
	}
}

// TestCustomAccessTokenHook_NonAdminUserOmitsRoleClaim verifies that a user
// with no admin role assignment has no "role" key in app_metadata.
func TestCustomAccessTokenHook_NonAdminUserOmitsRoleClaim(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	t.Cleanup(func() { db.Close() })

	userID := insertAuthUserForAdmin(t, ctx, db)
	sqlDB := sqlDBForTest(t, db)

	result := callCustomAccessTokenHook(t, ctx, sqlDB, userID, map[string]any{
		"app_metadata": map[string]any{},
	})
	claims := claimsFromHookResult(t, result)
	meta := appMetadataFromClaims(t, claims)

	if _, present := meta["role"]; present {
		t.Fatalf("app_metadata.role should be absent for non-admin user, got: %v", meta)
	}
}

// TestCustomAccessTokenHook_PreservesOtherClaims verifies that the hook never
// drops or rewrites sibling claims supplied by GoTrue. Only app_metadata.role
// is touched; sub, app_metadata.provider, user_metadata.name all survive.
func TestCustomAccessTokenHook_PreservesOtherClaims(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	t.Cleanup(func() { db.Close() })

	userID := insertAuthUserForAdmin(t, ctx, db)
	sqlDB := sqlDBForTest(t, db)

	result := callCustomAccessTokenHook(t, ctx, sqlDB, userID, map[string]any{
		"sub": "abc",
		"app_metadata": map[string]any{
			"provider": "google",
		},
		"user_metadata": map[string]any{
			"name": "x",
		},
	})
	claims := claimsFromHookResult(t, result)

	sub, ok := claims["sub"].(string)
	if !ok || sub != "abc" {
		t.Fatalf("claims.sub: got %v, want \"abc\"", claims["sub"])
	}

	meta := appMetadataFromClaims(t, claims)
	provider, ok := meta["provider"].(string)
	if !ok || provider != "google" {
		t.Fatalf("claims.app_metadata.provider: got %v, want \"google\"", meta["provider"])
	}

	rawUserMeta, ok := claims["user_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("claims.user_metadata is not an object: %T", claims["user_metadata"])
	}
	name, ok := rawUserMeta["name"].(string)
	if !ok || name != "x" {
		t.Fatalf("claims.user_metadata.name: got %v, want \"x\"", rawUserMeta["name"])
	}
}

// TestCustomAccessTokenHook_RemovesStaleRoleForNonAdmin verifies that a stale
// "role": "admin" claim is cleared when the user no longer has the admin role.
// This protects against a previously-admin user keeping the token-side claim
// after revocation in public.user_roles.
func TestCustomAccessTokenHook_RemovesStaleRoleForNonAdmin(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	t.Cleanup(func() { db.Close() })

	userID := insertAuthUserForAdmin(t, ctx, db)
	sqlDB := sqlDBForTest(t, db)

	result := callCustomAccessTokenHook(t, ctx, sqlDB, userID, map[string]any{
		"app_metadata": map[string]any{
			"role": "admin",
		},
	})
	claims := claimsFromHookResult(t, result)
	meta := appMetadataFromClaims(t, claims)

	if _, present := meta["role"]; present {
		t.Fatalf("app_metadata.role should be cleared for non-admin user, got: %v", meta)
	}
}

// callCustomAccessTokenHookRaw invokes public.custom_access_token_hook(event)
// with a raw JSON payload and returns the resulting JSON bytes and the SQL
// error (if any). Unlike callCustomAccessTokenHook, it does not t.Fatal on
// SQL failure — malformed-event tests need to assert on the error.
func callCustomAccessTokenHookRaw(t *testing.T, ctx context.Context, sqlDB *sql.DB, eventJSON []byte) ([]byte, error) {
	t.Helper()
	var resultJSON []byte
	err := sqlDB.QueryRowContext(ctx,
		`SELECT public.custom_access_token_hook($1::jsonb)`, eventJSON).Scan(&resultJSON)
	return resultJSON, err
}

// TestCustomAccessTokenHook_RejectsEventWithoutUserId verifies that the
// function raises an exception when the event payload omits the user_id key.
// The guard runs before any DB read, so no user fixture is required.
func TestCustomAccessTokenHook_RejectsEventWithoutUserId(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	t.Cleanup(func() { db.Close() })

	sqlDB := sqlDBForTest(t, db)

	eventJSON := []byte(`{"claims": {}}`)
	_, err := callCustomAccessTokenHookRaw(t, ctx, sqlDB, eventJSON)
	if err == nil {
		t.Fatalf("expected error for event without user_id, got nil")
	}
	if !strings.Contains(err.Error(), "malformed event") {
		t.Fatalf("expected error to mention \"malformed event\", got: %v", err)
	}
}

// TestCustomAccessTokenHook_RejectsEventWithoutClaims verifies that the
// function raises an exception when the event payload omits the claims key.
// The guard runs before any DB read, so no user fixture is required.
func TestCustomAccessTokenHook_RejectsEventWithoutClaims(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	t.Cleanup(func() { db.Close() })

	sqlDB := sqlDBForTest(t, db)

	eventJSON := []byte(`{"user_id": "00000000-0000-0000-0000-000000000000"}`)
	_, err := callCustomAccessTokenHookRaw(t, ctx, sqlDB, eventJSON)
	if err == nil {
		t.Fatalf("expected error for event without claims, got nil")
	}
	if !strings.Contains(err.Error(), "malformed event") {
		t.Fatalf("expected error to mention \"malformed event\", got: %v", err)
	}
}

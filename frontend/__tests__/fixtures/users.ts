/**
 * Shared user and role fixtures for broad page-level tests.
 *
 * All IDs are deterministic strings so snapshot assertions remain stable.
 * No runtime randomness or dynamic dates are used here.
 */

import type { Role, User } from "@/generated/graphql";

// ---------------------------------------------------------------------------
// Role fixtures
// ---------------------------------------------------------------------------

/** System admin role — protected; cannot be deleted or reassigned away from the last admin. */
export const adminRoleFixture: Role = {
  __typename: "Role",
  id: "role-admin",
  name: "admin",
};

/** Default role assigned to regular authenticated users. */
export const generalRoleFixture: Role = {
  __typename: "Role",
  id: "role-general",
  name: "general",
};

/** A non-system editor role used for multi-select and CRUD tests. */
export const editorRoleFixture: Role = {
  __typename: "Role",
  id: "role-editor",
  name: "editor",
};

// ---------------------------------------------------------------------------
// User fixtures
// ---------------------------------------------------------------------------

/** A user that holds the admin role — for tests that require an admin actor. */
export const adminUserFixture: User = {
  __typename: "User",
  id: "user-admin-1",
  displayName: "Admin User",
  bio: null,
  avatarUrl: null,
  roles: [adminRoleFixture],
};

/** A user that holds only the general role — the common non-privileged case. */
export const generalUserFixture: User = {
  __typename: "User",
  id: "user-general-1",
  displayName: "General User",
  bio: null,
  avatarUrl: null,
  roles: [generalRoleFixture],
};

/** A user with no roles at all — exercises the "no role" edge-case path. */
export const userWithoutRolesFixture: User = {
  __typename: "User",
  id: "user-noroles-1",
  displayName: "No Roles User",
  bio: null,
  avatarUrl: null,
  roles: [],
};

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/**
 * Builds an ad-hoc User by merging caller-supplied overrides onto a baseline.
 *
 * The baseline ID is "user-custom-1"; supply `id` in overrides to
 * differentiate multiple instances in the same test.
 */
export function makeUser(overrides: Partial<User> = {}): User {
  const id = overrides.id ?? "user-custom-1";
  return {
    __typename: "User",
    id,
    displayName: `User ${id}`,
    bio: null,
    avatarUrl: null,
    roles: [],
    ...overrides,
  };
}

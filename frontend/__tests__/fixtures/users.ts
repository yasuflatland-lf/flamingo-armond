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
const adminRoleFixture: Role = {
  __typename: "Role",
  id: "role-admin",
  name: "admin",
};

// ---------------------------------------------------------------------------
// User fixtures
// ---------------------------------------------------------------------------

/** A user that holds the admin role — for tests that require an admin actor. */
export const adminUserFixture: User = {
  __typename: "User",
  id: "user-admin-1",
  version: 0,
  displayName: "Admin User",
  bio: null,
  avatarUrl: null,
  roles: [adminRoleFixture],
};

/** A user that holds only the general role — the common non-privileged case. */
export const generalUserFixture: User = {
  __typename: "User",
  id: "user-general-1",
  version: 0,
  displayName: "General User",
  bio: null,
  avatarUrl: null,
  roles: [{ __typename: "Role", id: "role-general", name: "general" }],
};

/** A user with no roles at all — exercises the "no role" edge-case path. */
export const userWithoutRolesFixture: User = {
  __typename: "User",
  id: "user-noroles-1",
  version: 0,
  displayName: "No Roles User",
  bio: null,
  avatarUrl: null,
  roles: [],
};

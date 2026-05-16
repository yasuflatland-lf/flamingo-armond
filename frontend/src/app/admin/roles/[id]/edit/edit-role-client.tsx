"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { RoleForm } from "@/components/admin/role-form";
import { Button } from "@/components/ui/button";
import { AdminUpdateRoleMutation, SYSTEM_ROLE_NAMES } from "../../queries";

export type RoleForEdit = { id: string; name: string };

type Props = {
  role: RoleForEdit;
};

// Lift GraphQL extension codes from an arbitrary Apollo / network error for
// the warn payload. We do NOT trust err.message — backend messages may echo
// user input — but extensions.code is a fixed enum from the server's
// resolver layer and safe to log. The shape check is permissive: if the
// underlying transport surfaces graphQLErrors as a property on the Error,
// pluck them; otherwise return an empty list. We deliberately do not
// reuse parseGqlErrors here — that helper is keyed on the literal
// "GraphQL errors: " message prefix, which not every transport produces.
function liftGraphQLCodes(err: unknown): string[] {
  if (err == null || typeof err !== "object") return [];
  const maybe = (err as { graphQLErrors?: unknown }).graphQLErrors;
  if (!Array.isArray(maybe)) return [];
  const codes: string[] = [];
  for (const entry of maybe) {
    const code = (entry as { extensions?: { code?: unknown } } | null | undefined)
      ?.extensions?.code;
    if (typeof code === "string") codes.push(code);
  }
  return codes;
}

export function EditRoleClient({ role }: Props) {
  const router = useRouter();
  const readOnly = SYSTEM_ROLE_NAMES.has(role.name);

  // Typed error from the CannotModifySystemRoleError union variant.
  // Cleared on each new submission attempt.
  const [systemRoleError, setSystemRoleError] = useState<string | null>(null);

  // Semantically distinct from systemRoleError: this surfaces a degraded
  // "Something went wrong" banner when the server returns a __typename the
  // client was not regenerated against, or null payload from a partial-
  // response null bubble. Separating these states keeps the typed-error
  // banner pinned to the actual typed error.
  const [unexpectedPayloadError, setUnexpectedPayloadError] = useState<string | null>(null);

  // Mid-session auth failures. Cleared on each new submission attempt so a
  // retry after re-login does not show a stale banner.
  // Per .claude/rules/frontend-rsc-error-handling.md § "Mid-session
  // UNAUTHENTICATED in a client component: degraded banner with
  // <Link href=\"/login\">, not redirect()".
  const [authError, setAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);

  // No optimisticResponse: CannotModifySystemRoleError is a typed union variant
  // and Apollo v3.x does not roll back optimistic writes on typed GraphQL errors
  // — see docs/pagination/drop-optimistic-response-typed-errors.md.
  const [updateRole, { loading, error }] = useMutation(AdminUpdateRoleMutation);

  async function handleSubmit(values: { name: string }) {
    setSystemRoleError(null);
    setUnexpectedPayloadError(null);
    setAuthError(null);
    // Mirror the backend `validateRoleName` normalization — see new-role-client.tsx.
    const name = values.name.trim().toLowerCase();
    // Mid-session auth failures surface via a dedicated banner with a
    // sign-in link; no redirect, per the rule referenced above.
    const result = await updateRole({ variables: { id: role.id, name } }).catch((err) => {
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setAuthError("unauthenticated");
        return null;
      }
      if (codes.includes("FORBIDDEN")) {
        setAuthError("forbidden");
        return null;
      }
      // err.message is omitted — backend messages may echo user input.
      // codes is safe to log (fixed enum of GraphQL extension codes).
      console.warn("[admin/roles/:id/edit] updateRole rejected", {
        roleId: role.id,
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });
    if (!result) return;
    const payload = result.data?.updateRole;
    // Capture __typename before the narrowing chain so the else branch can read
    // it without TypeScript narrowing the type to `never` at that point.
    const typename = (payload as { __typename?: string } | null | undefined)?.__typename ?? null;
    if (payload?.__typename === "CannotModifySystemRoleError") {
      // Domain invariant: system roles are immutable. Surface as a banner;
      // do not navigate and do not mutate the Apollo cache.
      setSystemRoleError(payload.message);
      return;
    }
    if (payload?.__typename === "UpdateRoleSuccess") {
      router.push("/admin/roles");
      router.refresh();
      return;
    }
    // Unknown variant: null payload, partial-response null bubble, or a future union
    // variant the client was not regenerated against. Warn loudly and show a degraded
    // banner — do not silently fall through into success-path code. Routed through a
    // distinct state from setSystemRoleError so the typed-error banner stays semantic.
    console.warn("[admin/roles/:id/edit] unexpected updateRole payload", {
      typename,
    });
    setUnexpectedPayloadError("Something went wrong. Please try again.");
  }

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Edit role</h1>

      {readOnly ? (
        <div
          role="status"
          data-testid="admin-role-edit-system-banner"
          className="mb-4 rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
        >
          This is a system role. Its name cannot be changed.
        </div>
      ) : null}

      {authError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-auth-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          <span>
            {authError === "unauthenticated"
              ? "Your session has expired. "
              : "You do not have permission. "}
          </span>
          <Link href="/login" className="underline">
            Sign in again
          </Link>
          .
        </div>
      ) : null}

      {systemRoleError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-system-role-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {systemRoleError}
        </div>
      ) : null}

      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-unexpected-payload-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
      ) : null}

      <RoleForm
        defaultValues={{ name: role.name }}
        submit={handleSubmit}
        submitLabel="Save"
        submitting={loading}
        error={error}
        readOnly={readOnly}
        secondarySlot={
          <Button asChild variant="outline">
            <Link href="/admin/roles">Cancel</Link>
          </Button>
        }
      />
    </main>
  );
}

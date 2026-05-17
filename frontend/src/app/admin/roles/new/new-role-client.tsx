"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { RoleForm } from "@/components/admin/role-form";
import { Button } from "@/components/ui/button";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { AdminCreateRoleMutation } from "../queries";

export function NewRoleClient() {
  const router = useRouter();

  // Typed InputValidationError variant — field-level message rendered next to
  // the offending input. Cleared on each new submission attempt.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Semantically distinct from validationError: this surfaces a degraded
  // "Something went wrong" banner when the server returns a __typename the
  // client was not regenerated against, or null payload from a partial-
  // response null bubble.
  const [unexpectedPayloadError, setUnexpectedPayloadError] = useState<string | null>(null);

  // Mid-session auth failures. Cleared on each new submission attempt so a
  // retry after re-login does not show a stale banner.
  // Per .claude/rules/frontend-rsc-error-handling.md § "Mid-session
  // UNAUTHENTICATED in a client component: degraded banner with
  // <Link href=\"/login\">, not redirect()".
  const [authError, setAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);

  // No optimisticResponse: typed errors (FORBIDDEN, InputValidationError for
  // duplicate name) can fail the mutation, and Apollo does not consistently
  // roll back optimistic writes for typed GraphQL errors. The user pays one
  // round-trip of latency in exchange for a truthful cache.
  const [createRole, { loading }] = useMutation(AdminCreateRoleMutation);

  async function handleSubmit(values: { name: string }) {
    setValidationError(null);
    setUnexpectedPayloadError(null);
    setAuthError(null);
    // Mirror the backend `validateRoleName` normalization so the mutation
    // always sees the canonical form. TanStack Form's `value` is the raw
    // user input — Zod's `transform` runs in validators only, not on submit.
    const name = values.name.trim().toLowerCase();
    const result = await createRole({ variables: { name } }).catch((err) => {
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
      console.warn("[admin/roles/new] createRole rejected", {
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });
    if (!result) return;
    const payload = result.data?.createRole;
    // Capture typename before narrowing so the unknown-variant branch still
    // has access to it (TypeScript narrows to `never` after the known cases).
    const typename = payload?.__typename ?? null;
    if (payload?.__typename === "InputValidationError") {
      setValidationError({ field: payload.field, message: payload.message });
      return;
    }
    if (payload?.__typename === "CreateRoleSuccess") {
      router.push("/admin/roles");
      router.refresh();
      return;
    }
    // Unknown variant: null payload, partial-response null bubble, or a future
    // union variant the client was not regenerated against. Warn loudly and
    // show a degraded banner.
    console.warn("[admin/roles/new] unexpected createRole payload", {
      typename,
    });
    setUnexpectedPayloadError("Something went wrong. Please try again.");
  }

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">New role</h1>

      {authError ? (
        <div
          role="alert"
          data-testid="admin-role-new-auth-error"
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

      {validationError ? (
        <div
          role="alert"
          data-testid="admin-role-new-validation-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {validationError.message}
        </div>
      ) : null}

      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="admin-role-new-unexpected-payload-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
      ) : null}

      <RoleForm
        defaultValues={{ name: "" }}
        submit={handleSubmit}
        submitLabel="Create"
        submitting={loading}
        secondarySlot={
          <Button asChild variant="outline">
            <Link href="/admin/roles">Cancel</Link>
          </Button>
        }
      />
    </main>
  );
}

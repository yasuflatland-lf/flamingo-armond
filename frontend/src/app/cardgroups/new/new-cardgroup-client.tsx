"use client";

import { useMutation } from "@apollo/client/react";
import { Sparkles } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { CARDGROUPS_DEFAULT_VARS, CreateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

interface NewCardgroupClientProps {
  showWelcome?: boolean;
  /** Sanitized internal path to return to after creation, or null for the default redirect. */
  returnTo: string | null;
}

export function NewCardgroupClient({ showWelcome = false, returnTo }: NewCardgroupClientProps) {
  const router = useRouter();

  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server. Cleared on each new submission attempt.
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
  // <Link href="/login">, not redirect()".
  const [authError, setAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);

  // No optimisticResponse: typed errors (FORBIDDEN, InputValidationError for
  // duplicate name) can fail the mutation, and Apollo does not consistently
  // roll back optimistic writes for typed GraphQL errors. The user pays one
  // round-trip of latency in exchange for a truthful cache.
  const [createCardgroup, { loading }] = useMutation(CreateCardgroupMutation, {
    update(cache, { data }) {
      // Narrow on __typename before accessing .cardgroup so an InputValidationError
      // or unknown variant does not silently mutate the cache.
      if (data?.createCardgroup?.__typename !== "CreateCardgroupSuccess") return;
      const created = data.createCardgroup.cardgroup;

      // Update the Connection cache so the /cardgroups listing page
      // shows the new entry without a refetch when the user returns there.
      // cache.modify is forbidden — use readQuery + writeQuery so cold-cache
      // entries are also handled correctly. See
      // docs/pagination/cache-modify-skips-nonexistent-fields.md.
      // CARDGROUPS_DEFAULT_VARS keeps the cache key in sync with the SSR seed
      // and the client useQuery — any mismatch makes this write invisible.
      const existingConnection = cache.readQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      });
      const newEdge = {
        __typename: "CardgroupEdge" as const,
        cursor: created.id,
        node: created,
      };
      const nextConnection = existingConnection
        ? {
            ...existingConnection.myCardgroupsConnection,
            edges: [newEdge, ...existingConnection.myCardgroupsConnection.edges],
            totalCount: existingConnection.myCardgroupsConnection.totalCount + 1,
          }
        : {
            // Cold cache: build a minimal connection so the listing page can render
            // the new edge immediately when the user lands there.
            __typename: "CardgroupConnection" as const,
            edges: [newEdge],
            pageInfo: {
              __typename: "PageInfo" as const,
              hasNextPage: false,
              hasPreviousPage: false,
              startCursor: created.id,
              endCursor: created.id,
            },
            totalCount: 1,
          };
      cache.writeQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
        data: { myCardgroupsConnection: nextConnection },
      });
    },
  });

  async function handleSubmit(values: { name: string }) {
    setValidationError(null);
    setUnexpectedPayloadError(null);
    setAuthError(null);

    const result = await createCardgroup({
      variables: { input: { name: values.name } },
    }).catch((err) => {
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
      console.warn("[cardgroups-new] createCardgroup rejected", {
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });

    if (!result) return;

    const payload = result.data?.createCardgroup;
    // Capture typename before narrowing so the unknown-variant branch still
    // has access to it (TypeScript narrows to `never` after the known cases).
    const typename = payload?.__typename ?? null;

    if (payload?.__typename === "InputValidationError") {
      setValidationError({ field: payload.field, message: payload.message });
      return;
    }

    if (payload?.__typename === "CreateCardgroupSuccess") {
      const created = payload.cardgroup;
      if (returnTo) {
        const sep = returnTo.includes("?") ? "&" : "?";
        router.push(`${returnTo}${sep}cardgroup=${created.id}`);
        return;
      }
      router.push(`/cardgroups/${created.id}`);
      router.refresh();
      return;
    }

    // Unknown variant: null payload, partial-response null bubble, or a future
    // union variant the client was not regenerated against. Warn loudly and
    // show a degraded banner.
    console.warn("[cardgroups-new] unexpected createCardgroup payload", {
      typename,
    });
    setUnexpectedPayloadError("Something went wrong. Please try again.");
  }

  return (
    <main className="p-8">
      {showWelcome ? (
        <section
          aria-labelledby="welcome-heading"
          className="mb-8 rounded-2xl border border-brand-tint-border bg-brand-tint p-6 sm:p-8"
        >
          <div className="flex items-start gap-4">
            <Sparkles aria-hidden className="mt-1 h-6 w-6 shrink-0 text-brand-primary" />
            <div>
              <h1 id="welcome-heading" className="text-lg font-semibold tracking-tight">
                Welcome! Let's create your first cardgroup.
              </h1>
              <p className="mt-1 text-sm text-muted-foreground">
                A cardgroup holds the cards you study together &mdash; you can add more anytime.
              </p>
            </div>
          </div>
        </section>
      ) : (
        <div className="mb-6 flex items-center gap-4">
          <Link href="/cardgroups" className="text-sm text-muted-foreground hover:underline">
            &larr; Back
          </Link>
          <h1 className="text-2xl font-semibold">New cardgroup</h1>
        </div>
      )}

      {authError ? (
        <div
          role="alert"
          data-testid="cardgroup-new-auth-error"
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
          data-testid="cardgroup-new-validation-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {validationError.message}
        </div>
      ) : null}

      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="cardgroup-new-unexpected-payload-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
      ) : null}

      <CardgroupForm
        mode="create"
        defaultValues={{ name: "" }}
        submit={handleSubmit}
        submitting={loading}
        validationError={validationError}
      />
    </main>
  );
}

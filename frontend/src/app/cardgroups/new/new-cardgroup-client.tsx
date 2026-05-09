"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { CARDGROUPS_DEFAULT_VARS, CreateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { MyCardgroupsConnectionDocument, MyCardgroupsDocument } from "@/generated/graphql";

interface NewCardgroupClientProps {
  showWelcome?: boolean;
  /** Sanitized internal path to return to after creation, or null for the default redirect. */
  returnTo: string | null;
}

export function NewCardgroupClient({ showWelcome = false, returnTo }: NewCardgroupClientProps) {
  const router = useRouter();

  const [createCardgroup, { loading, error }] = useMutation(CreateCardgroupMutation, {
    update(cache, { data }) {
      if (!data?.createCardgroup?.cardgroup) return;
      const created = data.createCardgroup.cardgroup;

      // Update the deprecated flat list cache (still consumed by the cardgroup
      // picker sheet and a few other call sites).
      const existing = cache.readQuery({ query: MyCardgroupsDocument });
      cache.writeQuery({
        query: MyCardgroupsDocument,
        data: { myCardgroups: [created, ...(existing?.myCardgroups ?? [])] },
      });

      // Also update the Connection cache so the /cardgroups listing page
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
    onCompleted(data) {
      const created = data?.createCardgroup?.cardgroup;
      if (!created?.id) return;
      if (returnTo) {
        const sep = returnTo.includes("?") ? "&" : "?";
        router.push(`${returnTo}${sep}cardgroup=${created.id}`);
        return;
      }
      router.push(`/cardgroups/${created.id}`);
      router.refresh();
    },
  });

  async function handleSubmit(values: { name: string }) {
    await createCardgroup({
      variables: { input: { name: values.name } },
    }).catch((err) => {
      console.error("[cardgroups-new] mutation rejection", {
        message: err instanceof Error ? err.message : String(err),
        err,
      });
    });
  }

  return (
    <main className="p-8">
      {showWelcome && (
        <div className="mb-8 rounded-lg border border-border bg-card p-6">
          <h2 className="mb-2 text-lg font-semibold">
            Welcome! Let's create your first cardgroup.
          </h2>
          <p className="text-sm text-muted-foreground">
            A cardgroup holds the cards you want to study together. You can always add more cards
            later.
          </p>
        </div>
      )}
      <div className="mb-6 flex items-center gap-4">
        <Link href="/cardgroups" className="text-sm text-muted-foreground hover:underline">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-semibold">New cardgroup</h1>
      </div>
      <CardgroupForm
        mode="create"
        defaultValues={{ name: "" }}
        submit={handleSubmit}
        submitting={loading}
        error={error}
      />
    </main>
  );
}

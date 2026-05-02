"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { CreateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { MyCardgroupsDocument } from "@/generated/graphql";

interface NewCardgroupClientProps {
  showWelcome?: boolean;
}

export function NewCardgroupClient({ showWelcome = false }: NewCardgroupClientProps) {
  const router = useRouter();

  const [createCardgroup, { loading, error }] = useMutation(CreateCardgroupMutation, {
    update(cache, { data }) {
      if (!data?.createCardgroup?.cardgroup) return;
      const existing = cache.readQuery({ query: MyCardgroupsDocument });
      cache.writeQuery({
        query: MyCardgroupsDocument,
        data: {
          myCardgroups: [data.createCardgroup.cardgroup, ...(existing?.myCardgroups ?? [])],
        },
      });
    },
    onCompleted(data) {
      if (!data?.createCardgroup?.cardgroup?.id) return;
      router.push(`/cardgroups/${data.createCardgroup.cardgroup.id}`);
      router.refresh();
    },
  });

  async function handleSubmit(values: { name: string }) {
    await createCardgroup({
      variables: { input: { name: values.name } },
    }).catch((err) => {
      console.error("[NewCardgroupClient] mutation rejection", err);
    });
  }

  return (
    <main className="mx-auto max-w-xl p-8">
      {showWelcome && (
        <div className="mb-8 rounded-lg border border-border bg-card p-6">
          <h2 className="mb-2 text-lg font-semibold">Welcome! Let's create your first cardgroup.</h2>
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

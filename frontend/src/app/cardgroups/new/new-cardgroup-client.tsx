"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { CreateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { MyCardgroupsDocument } from "@/generated/graphql";

export function NewCardgroupClient() {
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

import { redirect } from "next/navigation";
import { CardgroupQuery } from "@/app/cardgroups/queries";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { EditCardgroupClient } from "./edit-cardgroup-client";

type Props = {
  params: Promise<{ id: string }>;
};

export default async function EditCardgroupPage({ params }: Props) {
  const { id } = await params;

  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr) throw authErr;
  if (!user) redirect("/cardgroups");

  let data: { cardgroup?: { id: string; name: string; updatedAt: unknown } | null };
  try {
    data = await gqlFetch(CardgroupQuery, { variables: { id }, revalidate: 0 });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("UNAUTHENTICATED")) redirect("/cardgroups");
    throw err;
  }

  const cg = data.cardgroup;
  if (!cg) redirect("/cardgroups");

  return <EditCardgroupClient cardgroup={{ id: cg.id, name: cg.name }} />;
}

import Link from "next/link";
import { redirect } from "next/navigation";
import { Plus } from "lucide-react";
import { CardgroupListItem } from "@/components/cardgroups/cardgroup-list-item";
import type { MyCardgroupsQuery as MyCardgroupsQueryType } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { MyCardgroupsQuery } from "./queries";

export default async function CardgroupsPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[cardgroups] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  let data: MyCardgroupsQueryType;
  try {
    data = await gqlFetch(MyCardgroupsQuery, { revalidate: 0 });
  } catch (err) {
    redirectIfUnauthenticated(err, "/login");
  }
  const cardgroups = data.myCardgroups;

  return (
    <main className="mx-auto max-w-2xl p-8">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">My Cardgroups</h1>
      </div>

      {cardgroups.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-8 text-center">
          <p className="mb-4 text-muted-foreground">You haven&apos;t created any cardgroups yet.</p>
          <Link
            href="/cardgroups/new"
            className="rounded-md bg-brand-primary px-4 py-2 text-sm font-medium text-brand-primary-foreground hover:opacity-90 transition-opacity"
          >
            New cardgroup
          </Link>
        </div>
      ) : (
        <>
          <ul className="space-y-2">
            {cardgroups.map((cg) => (
              <CardgroupListItem key={cg.id} id={cg.id} name={cg.name} updatedAt={cg.updatedAt} />
            ))}
          </ul>
          <div className="mt-6 border-t pt-4 text-center">
            <Link
              href="/cardgroups/new"
              className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground hover:underline"
            >
              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
              New cardgroup
            </Link>
          </div>
        </>
      )}
    </main>
  );
}

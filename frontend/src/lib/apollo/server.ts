import "server-only";
import type { TypedDocumentNode } from "@apollo/client";
import { print } from "graphql";
import { env } from "@/env";
import { createSupabaseServerClient } from "@/lib/supabase/server";

type GqlFetchInit<TVars> = {
  variables?: TVars;
  revalidate?: number | false;
};

export async function gqlFetch<TResult, TVars>(
  doc: TypedDocumentNode<TResult, TVars>,
  init: GqlFetchInit<TVars> = {},
): Promise<TResult> {
  const supabase = await createSupabaseServerClient();
  const {
    data: { session },
    error: sessionErr,
  } = await supabase.auth.getSession();
  if (sessionErr) throw sessionErr;

  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (session?.access_token) {
    headers.authorization = `Bearer ${session.access_token}`;
  }

  const res = await fetch(`${env.BACKEND_URL}/query`, {
    method: "POST",
    headers,
    body: JSON.stringify({ query: print(doc), variables: init.variables ?? {} }),
    // Pass `next` only when the caller explicitly sets revalidate. Omitting it
    // entirely lets Next.js apply its default; `0` opts out of caching; `false`
    // caches indefinitely. These are three distinct states — don't collapse.
    next: init.revalidate === undefined ? undefined : { revalidate: init.revalidate },
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`GraphQL HTTP ${res.status} ${res.statusText}: ${body}`);
  }
  const json = (await res.json()) as { data?: TResult; errors?: unknown };
  if (json.errors) throw new Error(`GraphQL errors: ${JSON.stringify(json.errors)}`);
  if (!json.data) throw new Error("GraphQL response missing data");
  return json.data;
}

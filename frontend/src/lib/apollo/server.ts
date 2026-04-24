import "server-only";
// TODO(PR9): forward auth token via createSupabaseServerClient(cookies()) when `me` query lands.
import type { TypedDocumentNode } from "@apollo/client";
import { print } from "graphql";
import { env } from "@/env";

type GqlFetchInit<TVars> = {
  variables?: TVars;
  revalidate?: number | false;
};

export async function gqlFetch<TResult, TVars>(
  doc: TypedDocumentNode<TResult, TVars>,
  init: GqlFetchInit<TVars> = {},
): Promise<TResult> {
  const res = await fetch(`${env.BACKEND_URL}/query`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
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

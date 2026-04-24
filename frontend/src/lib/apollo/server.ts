import "server-only";
import type { TypedDocumentNode } from "@apollo/client";
import { print } from "graphql";
import { env } from "@/env";

type GqlFetchInit = {
  variables?: Record<string, unknown>;
  revalidate?: number | false;
};

export async function gqlFetch<TResult, TVars>(
  doc: TypedDocumentNode<TResult, TVars>,
  init: GqlFetchInit = {},
): Promise<TResult> {
  const res = await fetch(`${env.BACKEND_URL}/query`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query: print(doc), variables: init.variables ?? {} }),
    next: init.revalidate === undefined ? undefined : { revalidate: init.revalidate },
  });
  if (!res.ok) throw new Error(`GraphQL HTTP ${res.status}`);
  const json = (await res.json()) as { data?: TResult; errors?: unknown };
  if (json.errors) throw new Error(`GraphQL errors: ${JSON.stringify(json.errors)}`);
  if (!json.data) throw new Error("GraphQL response missing data");
  return json.data;
}

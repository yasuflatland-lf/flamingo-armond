import "server-only";
import type { TypedDocumentNode } from "@apollo/client";
import { print } from "graphql";
import { env } from "@/env";
import { newRequestId, REQUEST_ID_HEADER } from "@/lib/observability/request-id";
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

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "application/graphql-response+json, application/json;q=0.9",
  };
  if (session?.access_token) {
    headers.authorization = `Bearer ${session.access_token}`;
  }
  // Always assign — gqlFetch is the RSC entrypoint and has no caller-supplied headers.
  headers[REQUEST_ID_HEADER] = newRequestId();

  const res = await fetch(`${env.BACKEND_URL}/query`, {
    method: "POST",
    headers,
    body: JSON.stringify({ query: print(doc), variables: init.variables ?? {} }),
    // Three distinct states for `next`: undefined keeps Next.js defaults,
    // `{ revalidate: 0 }` opts out of caching, `{ revalidate: false }` caches indefinitely.
    next: init.revalidate === undefined ? undefined : { revalidate: init.revalidate },
  });
  if (!res.ok) {
    const body = await res
      .text()
      .catch((e) => `<unreadable body: ${e instanceof Error ? e.message : String(e)}>`);
    throw new Error(`GraphQL HTTP ${res.status} ${res.statusText}: ${body}`);
  }
  const json = (await res.json()) as { data?: TResult; errors?: unknown };
  if (json.errors) {
    if (json.data != null) {
      // Partial response: data is present alongside errors (GraphQL over HTTP §5.2).
      // Return data so callers can use what the server provided; warn for debuggability.
      console.warn("[gqlFetch] partial response with errors:", JSON.stringify(json.errors));
      return json.data;
    }
    throw new Error(`GraphQL errors: ${JSON.stringify(json.errors)}`);
  }
  if (!json.data) throw new Error("GraphQL response missing data");
  return json.data;
}

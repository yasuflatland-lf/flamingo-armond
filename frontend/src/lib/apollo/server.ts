import "server-only";
import type { TypedDocumentNode } from "@apollo/client";
import { addTypenameToDocument } from "@apollo/client/utilities";
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
    body: JSON.stringify({
      // Request `__typename` on every non-root selection, matching what Apollo
      // Client's links add at runtime on the browser path. graphql-codegen's
      // client-preset (v6) does not inject `__typename` into the document, and
      // this RSC path prints the raw document instead of running it through
      // Apollo's links — so without this transform the SSR response carries no
      // `__typename`. When such a payload is seeded into the InMemoryCache via
      // `writeQuery`, the cache cannot resolve type-conditioned fragment fields
      // and silently drops them, surfacing downstream as e.g. "NaN cards" on
      // /catalog.
      query: print(addTypenameToDocument(doc)),
      variables: init.variables ?? {},
    }),
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
      // Partial response (GraphQL over HTTP §5.2): data arrived alongside errors.
      // Auth errors must still throw so RSC callers can redirect — silently returning
      // data would swallow the signal. Check before warn so auth re-throws never emit
      // a false-positive operator alert on clock-skew UNAUTHENTICATED events.
      const hasAuthError =
        Array.isArray(json.errors) &&
        json.errors.some((e: { extensions?: { code?: unknown } }) => {
          const code = e?.extensions?.code;
          return code === "UNAUTHENTICATED" || code === "FORBIDDEN";
        });
      if (hasAuthError) {
        throw new Error(`GraphQL errors: ${JSON.stringify(json.errors)}`);
      }
      console.warn("[gqlFetch] partial response with errors:", JSON.stringify(json.errors));
      return json.data;
    }
    throw new Error(`GraphQL errors: ${JSON.stringify(json.errors)}`);
  }
  if (!json.data) throw new Error("GraphQL response missing data");
  return json.data;
}

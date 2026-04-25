"use client";

import type { ApolloLink } from "@apollo/client";
import { setContext } from "@apollo/client/link/context";
import { newRequestId, REQUEST_ID_HEADER } from "@/lib/observability/request-id";

/**
 * Apollo link that injects a UUID v7 `X-Request-ID` header on every GraphQL
 * request. If an upstream link (or the caller) already set the header under
 * any capitalisation, the existing value is preserved so chained
 * server-to-server calls keep a single correlation ID across the full trace.
 */
export const requestIdLink: ApolloLink = setContext((_, { headers = {} }) => {
  const existing = headers as Record<string, string>;

  // Check case-insensitively whether the header is already present and non-empty.
  const headerLower = REQUEST_ID_HEADER.toLowerCase();
  const alreadySet = Object.keys(existing).some(
    (key) => key.toLowerCase() === headerLower && existing[key],
  );

  return {
    headers: alreadySet ? existing : { ...existing, [REQUEST_ID_HEADER]: newRequestId() },
  };
});

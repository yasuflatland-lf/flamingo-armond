"use client";

import { from, HttpLink } from "@apollo/client";
import { ApolloClient, InMemoryCache } from "@apollo/client-integration-nextjs";
import { makeApqLink } from "./apq-link";
import { authLink } from "./auth-link";
import { requestIdLink } from "./request-id-link";
import { makeRetryLink } from "./retry-link";

const httpLink = new HttpLink({
  uri: "/api/graphql",
  fetchOptions: { cache: "no-store" },
});

export function makeClient() {
  return new ApolloClient({
    cache: new InMemoryCache({
      typePolicies: {
        UserCardState: { keyFields: false },
      },
    }),
    // Placing retry before requestIdLink would split attempts across trace IDs;
    // placing it after authLink would replay stale auth, and after APQ would
    // give a PersistedQueryNotFound resend a fresh retry budget.
    link: from([requestIdLink, makeRetryLink(), authLink, makeApqLink(), httpLink]),
  });
}

"use client";

import { from, HttpLink } from "@apollo/client";
import { ApolloClient, InMemoryCache } from "@apollo/client-integration-nextjs";
import { makeApqLink } from "./apq-link";
import { authLink } from "./auth-link";

const httpLink = new HttpLink({
  uri: "/api/graphql",
  fetchOptions: { cache: "no-store" },
});

export function makeClient() {
  return new ApolloClient({
    cache: new InMemoryCache(),
    link: from([authLink, makeApqLink(), httpLink]),
  });
}

"use client";

import { ApolloNextAppProvider } from "@apollo/client-integration-nextjs";
import type { ReactNode } from "react";
import { makeClient } from "@/lib/apollo/client";

export function Providers({ children }: { children: ReactNode }) {
  return <ApolloNextAppProvider makeClient={makeClient}>{children}</ApolloNextAppProvider>;
}

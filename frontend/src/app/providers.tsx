"use client";

import { ApolloNextAppProvider } from "@apollo/client-integration-nextjs";
import type { ReactNode } from "react";
import { makeClient } from "@/lib/apollo/client";
import { UndoDeleteProvider } from "@/lib/undo-delete";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ApolloNextAppProvider makeClient={makeClient}>
      <UndoDeleteProvider>{children}</UndoDeleteProvider>
    </ApolloNextAppProvider>
  );
}

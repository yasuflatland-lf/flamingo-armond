"use client";

import { ApolloNextAppProvider } from "@apollo/client-integration-nextjs";
import type { ReactNode } from "react";
import { makeClient } from "@/lib/apollo/client";
import { UndoDeleteProvider } from "@/lib/undo-delete";

type ProvidersProps = {
  children: ReactNode;
  nonce?: string;
};

export function Providers({ children, nonce }: ProvidersProps) {
  return (
    <ApolloNextAppProvider
      makeClient={makeClient}
      extraScriptProps={nonce ? { nonce } : undefined}
    >
      <UndoDeleteProvider>{children}</UndoDeleteProvider>
    </ApolloNextAppProvider>
  );
}

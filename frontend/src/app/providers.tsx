"use client";

import { ApolloNextAppProvider } from "@apollo/client-integration-nextjs";
import type { ReactNode } from "react";
import { FabSuppressionProvider } from "@/components/nav/fab-suppression";
import { makeClient } from "@/lib/apollo/client";
import { UndoDeleteProvider } from "@/lib/undo-delete";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ApolloNextAppProvider makeClient={makeClient}>
      <FabSuppressionProvider>
        <UndoDeleteProvider>{children}</UndoDeleteProvider>
      </FabSuppressionProvider>
    </ApolloNextAppProvider>
  );
}

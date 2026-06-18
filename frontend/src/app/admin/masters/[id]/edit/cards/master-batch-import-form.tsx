"use client";

import { useMutation } from "@apollo/client/react";
import type { JSX } from "react";
import {
  BatchImportWizard,
  type ImportResult,
} from "@/components/batch-import/batch-import-wizard";
import { AdminMasterCardsConnectionDocument } from "@/generated/graphql";
import { AdminImportMasterCards } from "./queries";

/**
 * Master-flavored batch import: supplies the admin master-cards import mutation and
 * the admin master-cards refetch document to the shared two-step {@link BatchImportWizard}.
 */
export function MasterBatchImportForm(props: {
  masterId: string;
  deckName: string;
  onImported?: () => void;
  onCancel?: () => void;
}): JSX.Element {
  const { masterId, deckName, onImported, onCancel } = props;
  const [runImport, { loading: importing }] = useMutation(AdminImportMasterCards);

  async function onImport(payload: string): Promise<ImportResult | null> {
    const result = await runImport({
      variables: { input: { masterCardgroupId: masterId, payload } },
    });
    return result.data?.adminImportMasterCards ?? null;
  }

  return (
    <BatchImportWizard
      onImport={onImport}
      importing={importing}
      refetchDocument={AdminMasterCardsConnectionDocument}
      targetId={masterId}
      targetName={deckName}
      onImported={onImported}
      onCancel={onCancel}
    />
  );
}

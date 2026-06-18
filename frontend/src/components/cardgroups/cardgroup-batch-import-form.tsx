"use client";

import { useMutation } from "@apollo/client/react";
import type { JSX } from "react";
import { ImportCardsMutation } from "@/app/cardgroups/[id]/cards/queries";
import {
  BatchImportWizard,
  type ImportResult,
} from "@/components/batch-import/batch-import-wizard";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";

/**
 * Cardgroup-flavored batch import: supplies the cardgroup import mutation and the
 * cards-by-cardgroup refetch document to the shared two-step {@link BatchImportWizard}.
 */
export function CardgroupBatchImportForm(props: {
  cardgroupId: string;
  cardgroupName: string;
  onImported?: () => void;
  onCancel?: () => void;
}): JSX.Element {
  const { cardgroupId, cardgroupName, onImported, onCancel } = props;
  const [runImport, { loading: importing }] = useMutation(ImportCardsMutation);

  async function onImport(payload: string): Promise<ImportResult | null> {
    const result = await runImport({ variables: { input: { cardgroupId, payload } } });
    return result.data?.importCards ?? null;
  }

  return (
    <BatchImportWizard
      onImport={onImport}
      importing={importing}
      refetchDocument={CardsByCardgroupConnectionDocument}
      targetId={cardgroupId}
      targetName={cardgroupName}
      onImported={onImported}
      onCancel={onCancel}
    />
  );
}

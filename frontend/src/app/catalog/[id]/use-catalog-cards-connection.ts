import {
  CatalogMasterCardsConnectionDocument,
  type CatalogMasterCardsConnectionQuery,
  type CatalogMasterCardsConnectionQueryVariables,
} from "@/generated/graphql";
import type { UseConnectionPaginationResult } from "@/lib/pagination/use-connection-pagination";
import {
  defineEntityCardsConnectionConfig,
  type UseEntityCardsConnectionInput,
  useEntityCardsConnection,
} from "@/lib/pagination/use-entity-cards-connection";
import { catalogCardsDefaultVars } from "./queries";

type CatalogCardEdge = CatalogMasterCardsConnectionQuery["masterCardsConnection"]["edges"][number];
type CatalogCardPageInfo = CatalogMasterCardsConnectionQuery["masterCardsConnection"]["pageInfo"];

export interface UseCatalogCardsConnectionInput {
  masterCardgroupId: string;
  searchQuery: string | null;
  initialEdges: CatalogCardEdge[];
  initialPageInfo: CatalogCardPageInfo;
  initialTotalCount: number;
  /**
   * Localized fallback banner for a fetchMore failure with no backend-mapped
   * message. The hook is not a component and cannot call `useTranslations`, so
   * the client passes the localized string in.
   */
  fetchMoreErrorMessage: string;
}

export type UseCatalogCardsConnectionResult = UseConnectionPaginationResult<
  CatalogMasterCardsConnectionQuery,
  CatalogCardEdge,
  CatalogCardPageInfo,
  CatalogMasterCardsConnectionQueryVariables
>;

const CATALOG_CARDS_CONNECTION_CONFIG = defineEntityCardsConnectionConfig({
  document: CatalogMasterCardsConnectionDocument,
  connectionField: "masterCardsConnection",
  defaultVars: catalogCardsDefaultVars,
  logScope: "[catalog-deck]",
});

// Catalog deck-detail card pagination — a thin wrapper over the shared
// useEntityCardsConnection factory, mirroring `useCardsConnection`. The list is
// CARDS (not the /catalog cardgroups gallery), so the client passes a cards
// fetchMore fallback message; see `.claude/rules/pagination.md`.
export function useCatalogCardsConnection(
  input: UseCatalogCardsConnectionInput,
): UseCatalogCardsConnectionResult {
  const { masterCardgroupId, ...rest } = input;
  const factoryInput: UseEntityCardsConnectionInput<CatalogCardEdge, CatalogCardPageInfo> = {
    ownerId: masterCardgroupId,
    ...rest,
  };
  return useEntityCardsConnection<
    CatalogMasterCardsConnectionQuery,
    CatalogMasterCardsConnectionQueryVariables,
    "masterCardsConnection"
  >(CATALOG_CARDS_CONNECTION_CONFIG, factoryInput);
}

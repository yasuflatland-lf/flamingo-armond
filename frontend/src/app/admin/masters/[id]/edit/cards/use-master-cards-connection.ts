import {
  AdminMasterCardsConnectionDocument,
  type AdminMasterCardsConnectionQuery,
  type AdminMasterCardsConnectionQueryVariables,
} from "@/generated/graphql";
import type { UseConnectionPaginationResult } from "@/lib/pagination/use-connection-pagination";
import {
  defineEntityCardsConnectionConfig,
  type UseEntityCardsConnectionInput,
  useEntityCardsConnection,
} from "@/lib/pagination/use-entity-cards-connection";
import { masterCardsDefaultVars } from "./queries";

type MasterCardEdge =
  AdminMasterCardsConnectionQuery["adminMasterCardsConnection"]["edges"][number];
type MasterCardConnectionPageInfo =
  AdminMasterCardsConnectionQuery["adminMasterCardsConnection"]["pageInfo"];

export interface UseMasterCardsConnectionInput {
  masterId: string;
  searchQuery: string | null;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Localized banner copy shown when a load-more page fails with no backend
   * banner of its own. The hook has no `useTranslations`, so the caller passes
   * the resolved `t("fetchMoreFailed")` string in.
   */
  fetchMoreErrorMessage: string;
}

export type UseMasterCardsConnectionResult = UseConnectionPaginationResult<
  AdminMasterCardsConnectionQuery,
  MasterCardEdge,
  MasterCardConnectionPageInfo,
  AdminMasterCardsConnectionQueryVariables
>;

const MASTER_CARDS_CONNECTION_CONFIG = defineEntityCardsConnectionConfig({
  document: AdminMasterCardsConnectionDocument,
  connectionField: "adminMasterCardsConnection",
  defaultVars: masterCardsDefaultVars,
  logScope: "[master-cards-client]",
});

// Admin master-cards pagination — a thin wrapper over the shared
// useEntityCardsConnection factory. Binds the master-cards config and
// translates the route's master deck id to the factory's `ownerId`.
export function useMasterCardsConnection(
  input: UseMasterCardsConnectionInput,
): UseMasterCardsConnectionResult {
  const { masterId, ...rest } = input;
  const factoryInput: UseEntityCardsConnectionInput<MasterCardEdge, MasterCardConnectionPageInfo> =
    {
      ownerId: masterId,
      ...rest,
    };
  return useEntityCardsConnection<
    AdminMasterCardsConnectionQuery,
    AdminMasterCardsConnectionQueryVariables,
    "adminMasterCardsConnection"
  >(MASTER_CARDS_CONNECTION_CONFIG, factoryInput);
}

import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import type { UseConnectionPaginationResult } from "@/lib/pagination/use-connection-pagination";
import {
  defineEntityCardsConnectionConfig,
  type UseEntityCardsConnectionInput,
  useEntityCardsConnection,
} from "@/lib/pagination/use-entity-cards-connection";
import { cardsDefaultVars } from "./queries";

type CardEdge = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number];
type CardConnectionPageInfo =
  CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["pageInfo"];

export interface UseCardsConnectionInput {
  cardgroupId: string;
  searchQuery: string | null;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Localized fallback banner for a fetchMore failure with no backend-mapped
   * message. The hook is not a component and cannot call `useTranslations`, so
   * the client passes the localized string in.
   */
  fetchMoreErrorMessage: string;
}

export type UseCardsConnectionResult = UseConnectionPaginationResult<
  CardsByCardgroupConnectionQuery,
  CardEdge,
  CardConnectionPageInfo,
  CardsByCardgroupConnectionQueryVariables
>;

const CARDS_CONNECTION_CONFIG = defineEntityCardsConnectionConfig({
  document: CardsByCardgroupConnectionDocument,
  connectionField: "cardsByCardgroupConnection",
  defaultVars: cardsDefaultVars,
  logScope: "[cards-client]",
});

// Cards-by-cardgroup pagination — a thin wrapper over the shared
// useEntityCardsConnection factory. Binds the cards config and translates the
// route's `cardgroupId` to the factory's `ownerId`.
export function useCardsConnection(input: UseCardsConnectionInput): UseCardsConnectionResult {
  const { cardgroupId, ...rest } = input;
  const factoryInput: UseEntityCardsConnectionInput<CardEdge, CardConnectionPageInfo> = {
    ownerId: cardgroupId,
    ...rest,
  };
  return useEntityCardsConnection<
    CardsByCardgroupConnectionQuery,
    CardsByCardgroupConnectionQueryVariables,
    "cardsByCardgroupConnection"
  >(CARDS_CONNECTION_CONFIG, factoryInput);
}

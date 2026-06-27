import type { ApolloCache } from "@apollo/client";
import type { MyCardgroupsConnectionQuery } from "@/generated/graphql";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { appendConnectionEdge } from "@/lib/apollo/connection-cache";
import { CARDGROUPS_DEFAULT_VARS } from "./queries";

/**
 * The `node` projection carried by an edge of the `myCardgroupsConnection`
 * query — `{ __typename: "Cardgroup", id, name, updatedAt }`. Extracted from the
 * generated query type so callers pass a complete node and the helper stays in
 * sync with the document's selection set automatically.
 */
type MyCardgroupNode =
  MyCardgroupsConnectionQuery["myCardgroupsConnection"]["edges"][number]["node"];

/**
 * Prepend a freshly-created cardgroup as a new `{ cursor, node }` edge to the
 * cached `myCardgroupsConnection` and bump `totalCount`, so the new deck appears
 * on `/cardgroups` without a refetch.
 *
 * Shared by `useCreateCardgroup` (the `/cardgroups` feature), `useImportMaster`
 * (the `/catalog` feature, which seeds the cardgroups connection across features),
 * and `useSeedDefaultStarters` (onboarding). Each calls this from inside its
 * `__typename`-narrowed `update` callback after pulling the complete node off the
 * success payload.
 *
 * Delegates the warm-prepend / cold-build / dedup mechanics to the generic
 * `appendConnectionEdge`. The cold-cache build is supplied so a user landing on
 * `/cardgroups` or `/catalog` without an SSR seed still sees the new edge.
 * `CARDGROUPS_DEFAULT_VARS` keeps the cache key in sync with the `/cardgroups` SSR
 * seed and client `useQuery`; any mismatch makes this write invisible. See
 * .claude/rules/pagination.md.
 */
export function prependMyCardgroupEdge(cache: ApolloCache, node: MyCardgroupNode): void {
  appendConnectionEdge(cache, {
    document: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
    connectionField: "myCardgroupsConnection",
    edgeTypename: "CardgroupEdge",
    node,
    buildColdConnection: () => ({
      __typename: "CardgroupConnection" as const,
      edges: [{ __typename: "CardgroupEdge" as const, cursor: node.id, node }],
      pageInfo: {
        __typename: "PageInfo" as const,
        hasNextPage: false,
        hasPreviousPage: false,
        startCursor: node.id,
        endCursor: node.id,
      },
      totalCount: 1,
    }),
  });
}

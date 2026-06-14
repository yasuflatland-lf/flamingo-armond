import type { ApolloCache } from "@apollo/client";
import type { MyCardgroupsConnectionQuery } from "@/generated/graphql";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
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
 * Shared by `useCreateCardgroup` (the `/cardgroups` feature) and `useImportMaster`
 * (the `/catalog` feature, which seeds the cardgroups connection across features).
 * Both call this from inside their `__typename`-narrowed `update` callback after
 * pulling the complete node off the success payload.
 *
 * `cache.modify` is forbidden — `readQuery` + `writeQuery` handles the cold cache
 * (a user landing on `/cardgroups` or `/catalog` without an SSR seed) correctly.
 * `CARDGROUPS_DEFAULT_VARS` keeps the cache key in sync with the `/cardgroups` SSR
 * seed and client `useQuery`; any mismatch makes this write invisible. See
 * .claude/rules/pagination.md.
 *
 * No `cache.writeFragment` is needed: the appended node already carries its full
 * `{ __typename, id, name, updatedAt }` projection, so the `writeQuery` normalizes
 * it into the standalone `Cardgroup:<id>` entry on its own.
 */
export function prependMyCardgroupEdge(cache: ApolloCache, node: MyCardgroupNode): void {
  const existingConnection = cache.readQuery({
    query: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
  });
  const newEdge = {
    __typename: "CardgroupEdge" as const,
    cursor: node.id,
    node,
  };
  const nextConnection = existingConnection
    ? {
        ...existingConnection.myCardgroupsConnection,
        edges: [newEdge, ...existingConnection.myCardgroupsConnection.edges],
        totalCount: existingConnection.myCardgroupsConnection.totalCount + 1,
      }
    : {
        // Cold cache: build a minimal connection so the listing page can render
        // the new edge immediately when the user lands there.
        __typename: "CardgroupConnection" as const,
        edges: [newEdge],
        pageInfo: {
          __typename: "PageInfo" as const,
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: node.id,
          endCursor: node.id,
        },
        totalCount: 1,
      };
  cache.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
    data: { myCardgroupsConnection: nextConnection },
  });
}

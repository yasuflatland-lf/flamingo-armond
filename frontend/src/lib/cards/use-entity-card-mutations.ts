"use client";

import type { OperationVariables } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import { useCallback, useState } from "react";
import { prependConnectionEdge, removeConnectionEdges } from "@/lib/apollo/connection-cache";
import { useUndoDelete } from "@/lib/undo-delete";
import {
  type CardFormValues,
  type CreateCardOutcome,
  cardMatchesSearch,
  type UpdateCardOutcome,
} from "./card-mutation-outcomes";

/** Connection variables every card connection shares (search-filter aware). */
type CardConnectionVariables = OperationVariables & { search?: string | null };

/** Minimal node shape the cache writes + search predicate require. */
type CardNodeLike = { id: string; front: string; back: string };

/** Structural connection shape read during the optimistic per-row delete. */
type ConnectionSnapshot = {
  edges: { node: { id: string } }[];
  totalCount: number;
  [key: string]: unknown;
};

/** Classification of a create payload, produced by the typed wrapper. */
type CreateClassification<TNode extends CardNodeLike> =
  | { kind: "success"; node: TNode }
  | { kind: "duplicate"; message: string }
  | { kind: "unknown"; typename: string | null };

/** Classification of an update payload, produced by the typed wrapper. */
type UpdateClassification =
  | { kind: "success" }
  | { kind: "validation"; field: string; message: string }
  | { kind: "unknown"; typename: string | null };

export interface EntityCardMutationsConfig<
  TConnData,
  TConnVars extends CardConnectionVariables,
  TConnKey extends keyof TConnData,
  TNode extends CardNodeLike,
  TCreateData,
  TCreateVars extends OperationVariables,
  TUpdateData,
  TUpdateVars extends OperationVariables,
  TDeleteOneData,
  TDeleteManyData,
> {
  /** Log bracket tag, e.g. `"[useCardMutations]"`. */
  scope: string;
  /** Owner-id key used in structured log payloads, e.g. `"cardgroupId"`. */
  ownerLogKey: string;
  /** Operation name in the "unexpected <op> payload" warn, e.g. `"createCard"`. */
  createOpName: string;
  /** Operation name in the "unexpected <op> payload" warn, e.g. `"updateCard"`. */
  updateOpName: string;

  /** The connection query document, keyed for cache reads/writes. */
  connectionDocument: TypedDocumentNode<TConnData, TConnVars>;
  /** The connection field key on `TConnData`, e.g. `"cardsByCardgroupConnection"`. */
  connectionField: TConnKey;
  /** Edge `__typename`, e.g. `"CardEdge"`. */
  edgeTypename: string;
  /** Entity `__typename`, e.g. `"Card"`, used to evict the normalized entry. */
  entityTypename: string;
  /** Default connection vars factory, scoped by owner id. */
  defaultVars: (ownerId: string) => TConnVars;

  createDocument: TypedDocumentNode<TCreateData, TCreateVars>;
  buildCreateVariables: (ownerId: string, values: CardFormValues) => TCreateVars;
  classifyCreate: (data: TCreateData | null | undefined) => CreateClassification<TNode>;

  updateDocument: TypedDocumentNode<TUpdateData, TUpdateVars>;
  buildUpdateVariables: (id: string, values: CardFormValues) => TUpdateVars;
  classifyUpdate: (data: TUpdateData | null | undefined) => UpdateClassification;

  deleteOneDocument: TypedDocumentNode<TDeleteOneData, { id: string | number }>;
  extractDeleteOne: (data: TDeleteOneData | null | undefined) => boolean;

  deleteManyDocument: TypedDocumentNode<
    TDeleteManyData,
    { ids: Array<string | number> | string | number }
  >;
  extractDeleteManyCount: (data: TDeleteManyData | null | undefined) => number | null | undefined;
}

export interface UseEntityCardMutationsInput<TConnVars extends CardConnectionVariables> {
  ownerId: string;
  /**
   * The connection's active query variables (search-filter-aware). MUST be the
   * same object the live `useQuery` is keyed on so cache reads/writes land on
   * the right entry under any active search filter.
   * See docs/pagination/optimistic-rollback-cache-key.md.
   */
  queryVariables: TConnVars;
}

/**
 * Identity helper that supplies the contextual type for a config literal so the
 * wrapper's classifier/builder callbacks are fully type-checked, while keeping
 * the config a stable module constant. Prefer calling this WITHOUT explicit type
 * arguments and let inference resolve the params from the documents; supply
 * explicit type arguments only if `tsc` cannot infer one.
 */
export function defineEntityCardMutationsConfig<
  TConnData,
  TConnVars extends CardConnectionVariables,
  TConnKey extends keyof TConnData,
  TNode extends CardNodeLike,
  TCreateData,
  TCreateVars extends OperationVariables,
  TUpdateData,
  TUpdateVars extends OperationVariables,
  TDeleteOneData,
  TDeleteManyData,
>(
  config: EntityCardMutationsConfig<
    TConnData,
    TConnVars,
    TConnKey,
    TNode,
    TCreateData,
    TCreateVars,
    TUpdateData,
    TUpdateVars,
    TDeleteOneData,
    TDeleteManyData
  >,
): EntityCardMutationsConfig<
  TConnData,
  TConnVars,
  TConnKey,
  TNode,
  TCreateData,
  TCreateVars,
  TUpdateData,
  TUpdateVars,
  TDeleteOneData,
  TDeleteManyData
> {
  return config;
}

/**
 * Generic owner of the four card mutations (create / update / per-row delete /
 * bulk delete) and every Apollo cache write they perform, shared by the cards
 * and master-cards screens via thin typed wrappers. The wrapper supplies a
 * static `config` (documents, typenames, classifiers); this hook owns the
 * orchestration, the optimistic snapshot/restore, the undo-delete wiring, and
 * the structured logging.
 *
 * No `optimisticResponse`: typed errors (…DuplicateFrontError,
 * InputValidationError, UNAUTHENTICATED, FORBIDDEN) can fail these mutations and
 * Apollo does not reliably roll back optimistic writes for typed GraphQL errors
 * — see .claude/rules/pagination.md.
 */
export function useEntityCardMutations<
  TConnData,
  TConnVars extends CardConnectionVariables,
  TConnKey extends keyof TConnData,
  TNode extends CardNodeLike,
  TCreateData,
  TCreateVars extends OperationVariables,
  TUpdateData,
  TUpdateVars extends OperationVariables,
  TDeleteOneData,
  TDeleteManyData,
>(
  config: EntityCardMutationsConfig<
    TConnData,
    TConnVars,
    TConnKey,
    TNode,
    TCreateData,
    TCreateVars,
    TUpdateData,
    TUpdateVars,
    TDeleteOneData,
    TDeleteManyData
  >,
  input: UseEntityCardMutationsInput<TConnVars>,
) {
  const { ownerId, queryVariables } = input;
  const apollo = useApolloClient();
  const { scheduleDelete } = useUndoDelete();

  const [createMutation, { loading: creating, error: createError, reset: resetCreateCard }] =
    useMutation(config.createDocument);
  // Updates propagate automatically via Apollo cache normalization (entity has id).
  const [updateMutation, { loading: updating, error: updateError }] = useMutation(
    config.updateDocument,
  );
  // scheduleDelete (5s undo window) fires the per-row mutation imperatively.
  const [deleteOneMutation] = useMutation(config.deleteOneDocument);

  const [deleteManyMutation, { error: bulkDeleteError, loading: bulkDeleting }] = useMutation(
    config.deleteManyDocument,
    {
      // queryVariables matches the live cache entry under any active search filter.
      // See docs/pagination/optimistic-rollback-cache-key.md.
      update(cache, { data: bulkData }, { variables: mutationVars }) {
        const ids = mutationVars?.ids as string[] | undefined;
        if (!ids) return;
        const deletedCount = config.extractDeleteManyCount(bulkData);
        if (deletedCount == null) return;
        if (deletedCount === 0) return;
        removeConnectionEdges(cache, {
          document: config.connectionDocument,
          variables: queryVariables,
          connectionField: config.connectionField,
          entityTypename: config.entityTypename,
          ids,
          deletedCount,
        });
      },
    },
  );

  // Per-row delete commit error — surfaced after the 5s undo window elapses and
  // the DELETE mutation rejects. Stored as the raw error so the client derives
  // the localized banner copy; persists across mutation calls. See
  // docs/pagination/do-not-reuse-mutation-error-state.md.
  const [deleteRowError, setDeleteRowError] = useState<unknown>(null);

  // Write a freshly-created card to the connection cached under `variables`.
  // No `buildColdConnection`: on a cold cache this no-ops and the page's own
  // `useQuery` owns populating the connection.
  const writeCreatedToConnection = useCallback(
    (node: TNode, variables: TConnVars) => {
      prependConnectionEdge(apollo.cache, {
        document: config.connectionDocument,
        variables,
        connectionField: config.connectionField,
        edgeTypename: config.edgeTypename,
        node,
      });
    },
    [apollo, config],
  );

  const createCard = useCallback(
    async (values: CardFormValues): Promise<CreateCardOutcome> => {
      const result = await createMutation({
        variables: config.buildCreateVariables(ownerId, values),
      }).catch((err) => {
        console.error(`${config.scope} create rejection`, {
          name: err instanceof Error ? err.name : "unknown",
          [config.ownerLogKey]: ownerId,
        });
        return null;
      });
      if (!result) return { status: "rejected" };

      const classified = config.classifyCreate(result.data);
      if (classified.kind === "success") {
        writeCreatedToConnection(classified.node, config.defaultVars(ownerId));
        const search = queryVariables.search;
        if (typeof search === "string" && cardMatchesSearch(classified.node, search)) {
          writeCreatedToConnection(classified.node, queryVariables);
        }
        return { status: "success" };
      }
      if (classified.kind === "duplicate") {
        return { status: "validation", field: "front", message: classified.message };
      }
      console.warn(`${config.scope} unexpected ${config.createOpName} payload`, {
        typename: classified.typename,
        [config.ownerLogKey]: ownerId,
      });
      return { status: "unexpected" };
    },
    [config, ownerId, createMutation, queryVariables, writeCreatedToConnection],
  );

  const updateCard = useCallback(
    async (id: string, values: CardFormValues): Promise<UpdateCardOutcome> => {
      const result = await updateMutation({
        variables: config.buildUpdateVariables(id, values),
      }).catch((err) => {
        console.error(`${config.scope} update rejection`, {
          name: err instanceof Error ? err.name : "unknown",
          [config.ownerLogKey]: ownerId,
          cardId: id,
        });
        return null;
      });
      if (!result) return { status: "rejected" };

      const classified = config.classifyUpdate(result.data);
      if (classified.kind === "success") {
        return { status: "success" };
      }
      if (classified.kind === "validation") {
        return { status: "validation", field: classified.field, message: classified.message };
      }
      console.warn(`${config.scope} unexpected ${config.updateOpName} payload`, {
        typename: classified.typename,
        cardId: id,
        [config.ownerLogKey]: ownerId,
      });
      return { status: "unexpected" };
    },
    [config, ownerId, updateMutation],
  );

  // Per-row delete: snapshot, optimistic drop, schedule DELETE with a 5s undo
  // window. See docs/pagination/optimistic-rollback-cache-key.md for the
  // queryVariables rule. `label` is the caller-supplied (localized) toast title.
  const deleteRow = useCallback(
    (cardId: string, label: string) => {
      const snapshot = apollo.readQuery({
        query: config.connectionDocument,
        variables: queryVariables,
      });
      if (snapshot) {
        const current = snapshot[config.connectionField] as unknown as ConnectionSnapshot;
        apollo.writeQuery({
          query: config.connectionDocument,
          variables: queryVariables,
          data: {
            [config.connectionField]: {
              ...current,
              edges: current.edges.filter((edge) => edge.node.id !== cardId),
              totalCount: Math.max(0, current.totalCount - 1),
            },
          } as unknown as TConnData,
        });
      }
      scheduleDelete({
        id: cardId,
        label,
        optimisticRollback: () => {
          if (snapshot !== null) {
            apollo.writeQuery({
              query: config.connectionDocument,
              variables: queryVariables,
              data: snapshot,
            });
          }
          setDeleteRowError(null);
        },
        commitDelete: async () => {
          setDeleteRowError(null);
          const result = await deleteOneMutation({ variables: { id: cardId } });
          if (config.extractDeleteOne(result.data)) {
            apollo.cache.evict({
              id: apollo.cache.identify({ __typename: config.entityTypename, id: cardId }),
            });
            apollo.cache.gc();
          }
        },
        onCommitFailed: (err) => {
          setDeleteRowError(err);
        },
      });
    },
    [apollo, config, deleteOneMutation, queryVariables, scheduleDelete],
  );

  const deleteCards = useCallback(
    (ids: string[]) => deleteManyMutation({ variables: { ids } }),
    [deleteManyMutation],
  );

  return {
    createCard,
    updateCard,
    deleteRow,
    deleteCards,
    creating,
    updating,
    bulkDeleting,
    createError,
    updateError,
    bulkDeleteError,
    deleteRowError,
    resetCreateCard,
  };
}

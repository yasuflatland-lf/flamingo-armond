// @vitest-environment jsdom

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import {
  AdminCreateMasterCardDocument,
  AdminMasterCardsConnectionDocument,
  AdminUpdateMasterCardDocument,
} from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import enMessages from "../../../../../../../messages/en.json";
import { masterCardsDefaultVars } from "./queries";
import { useMasterCardMutations } from "./use-master-card-mutations";

const MASTER_ID = "m-1";

const baseSeed = {
  request: {
    query: AdminMasterCardsConnectionDocument,
    variables: masterCardsDefaultVars(MASTER_ID),
  },
  result: {
    data: {
      adminMasterCardsConnection: {
        __typename: "MasterCardConnection" as const,
        edges: [],
        pageInfo: {
          __typename: "PageInfo" as const,
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: null,
          endCursor: null,
        },
        totalCount: 0,
      },
    },
  },
};

function makeWrapper(extraMocks: object[]) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        <MockedProvider mocks={[baseSeed, ...extraMocks] as never}>
          <UndoDeleteProvider>{children}</UndoDeleteProvider>
        </MockedProvider>
      </NextIntlClientProvider>
    );
  };
}

describe("useMasterCardMutations", () => {
  it("maps MasterCardDuplicateFrontError to an inline front validation outcome", async () => {
    const wrapper = makeWrapper([
      {
        request: {
          query: AdminCreateMasterCardDocument,
          variables: { input: { masterCardgroupId: MASTER_ID, front: "dup", back: "b" } },
        },
        result: {
          data: {
            adminCreateMasterCard: {
              __typename: "MasterCardDuplicateFrontError",
              message: "already exists",
              existingCardId: "x-1",
              existingBack: "old",
            },
          },
        },
      },
    ]);
    const { result } = renderHook(
      () =>
        useMasterCardMutations({
          masterId: MASTER_ID,
          queryVariables: masterCardsDefaultVars(MASTER_ID),
        }),
      { wrapper },
    );
    let outcome: Awaited<ReturnType<typeof result.current.createCard>> | undefined;
    await act(async () => {
      outcome = await result.current.createCard({ front: "dup", back: "b" });
    });
    expect(outcome).toEqual({ status: "validation", field: "front", message: "already exists" });
  });

  it("maps InputValidationError on update to an inline validation outcome", async () => {
    const wrapper = makeWrapper([
      {
        request: {
          query: AdminUpdateMasterCardDocument,
          variables: { id: "c-1", input: { front: "", back: "b" } },
        },
        result: {
          data: {
            adminUpdateMasterCard: {
              __typename: "InputValidationError",
              field: "front",
              message: "front is required",
            },
          },
        },
      },
    ]);
    const { result } = renderHook(
      () =>
        useMasterCardMutations({
          masterId: MASTER_ID,
          queryVariables: masterCardsDefaultVars(MASTER_ID),
        }),
      { wrapper },
    );
    let outcome: Awaited<ReturnType<typeof result.current.updateCard>> | undefined;
    await act(async () => {
      outcome = await result.current.updateCard("c-1", { front: "", back: "b" });
    });
    expect(outcome).toEqual({ status: "validation", field: "front", message: "front is required" });
  });

  it("does not pass optimisticResponse as a mutation option (static-source guard)", () => {
    const src = readFileSync(
      join(process.cwd(), "src/app/admin/masters/[id]/edit/cards/use-master-card-mutations.ts"),
      "utf8",
    );
    // The wrapper now delegates to useEntityCardMutations, so the mutation
    // calls (and thus the real optimisticResponse risk) live in the generic
    // hook — its guard is in src/lib/cards/use-entity-card-mutations.test.tsx.
    // This assertion still pins the wrapper itself clean.
    expect(src).not.toMatch(/optimisticResponse\s*:/);
  });
});

// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
  }),
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
  usePathname: () => "/admin/masters/m-int-1/edit",
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));
vi.mock("@/lib/apollo/server", () => ({ gqlFetch: vi.fn() }));
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function Mock(
      { children }: { children: React.ReactNode },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return <div>{children}</div>;
    }),
  };
});

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { NextIntlClientProvider } from "next-intl";
import { masterCardsDefaultVars } from "@/app/admin/masters/[id]/edit/cards/queries";
import EditMasterPage from "@/app/admin/masters/[id]/edit/page";
import { AdminMasterCardsConnectionDocument } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import enMessages from "../messages/en.json";

const ID = "m-int-1";
const DECK = {
  __typename: "MasterCardgroup",
  id: ID,
  name: "Integration Deck",
  description: null,
  language: null,
  level: null,
  category: null,
  coverImageUrl: null,
  source: null,
  version: 1,
  status: "DRAFT",
  isDefaultStarter: false,
  sortOrder: 0,
  cardCount: 4,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

const connMock = {
  request: {
    query: AdminMasterCardsConnectionDocument,
    variables: masterCardsDefaultVars(ID),
  },
  result: {
    data: {
      adminMasterCardsConnection: {
        __typename: "MasterCardConnection",
        edges: [],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: null,
          endCursor: null,
        },
        totalCount: 4,
      },
    },
  },
};

async function renderPage() {
  const jsx = await EditMasterPage({ params: Promise.resolve({ id: ID }) });
  render(
    <MockedProvider mocks={[connMock]}>
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        <UndoDeleteProvider>{jsx as React.ReactElement}</UndoDeleteProvider>
      </NextIntlClientProvider>
    </MockedProvider>,
  );
}

beforeEach(() => vi.clearAllMocks());
afterEach(() => vi.clearAllMocks());

describe("EditMasterPage — broad integration", () => {
  it("renders the deck name as the h1 and mounts the cards section", async () => {
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ adminMaster: DECK } as never)
      .mockResolvedValueOnce({
        adminMasterCardsConnection: {
          edges: [],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: null,
            endCursor: null,
          },
          totalCount: 4,
        },
      } as never);
    await renderPage();
    expect(screen.getByRole("heading", { level: 1, name: "Integration Deck" })).toBeInTheDocument();
    expect(screen.getByTestId("master-cards-section")).toBeInTheDocument();
    expect(screen.getByTestId("master-edit-publish")).toBeInTheDocument();
  });

  it("redirects to / when not authenticated", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));
    await expect(EditMasterPage({ params: Promise.resolve({ id: ID }) })).rejects.toThrow(
      "REDIRECT:/",
    );
    expect(redirect).toHaveBeenCalledWith("/");
    expect(gqlFetch).not.toHaveBeenCalled();
  });
});

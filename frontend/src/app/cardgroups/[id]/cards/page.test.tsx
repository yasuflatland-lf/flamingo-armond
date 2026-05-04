// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// CardsClient is a "use client" component — stub it to keep the RSC test simple.
vi.mock("./cards-client", () => ({
  CardsClient: ({
    cardgroupId,
    initialEdges,
  }: {
    cardgroupId: string;
    initialEdges: { node: { front: string } }[];
    initialPageInfo: unknown;
    initialTotalCount: number;
  }) => (
    <div data-testid="cards-client" data-cardgroup-id={cardgroupId}>
      {initialEdges.map((e) => (
        <span key={e.node.front}>{e.node.front}</span>
      ))}
    </div>
  ),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import CardsPage from "./page";

function makeSupabaseMock(user: { id: string } | null) {
  return {
    auth: {
      getUser: vi.fn().mockResolvedValue({ data: { user }, error: null }),
    },
  };
}

const CARDGROUP = { id: "cg-1", name: "Vocab", updatedAt: "2024-06-15T10:00:00.000Z" };

const EDGES = [
  {
    cursor: "c-1",
    node: {
      id: "c-1",
      front: "Hello",
      back: "Hola",
      due: "2024-06-15",
      state: 0,
      cardgroupId: "cg-1",
    },
  },
  {
    cursor: "c-2",
    node: {
      id: "c-2",
      front: "World",
      back: "Mundo",
      due: "2024-06-15",
      state: 0,
      cardgroupId: "cg-1",
    },
  },
];

const PAGE_INFO = {
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: "c-1",
  endCursor: "c-2",
};

const CONNECTION = {
  cardsByCardgroupConnection: {
    edges: EDGES,
    pageInfo: PAGE_INFO,
    totalCount: EDGES.length,
  },
};

const EMPTY_CONNECTION = {
  cardsByCardgroupConnection: {
    edges: [],
    pageInfo: {
      hasNextPage: false,
      hasPreviousPage: false,
      startCursor: null,
      endCursor: null,
    },
    totalCount: 0,
  },
};

describe("CardsPage (RSC)", () => {
  it("redirects to /login when unauthenticated", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    await expect(CardsPage({ params: Promise.resolve({ id: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
  });

  it("redirects to /cardgroups when cardgroup not found", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({ cardgroup: null, ...EMPTY_CONNECTION } as never);

    await expect(CardsPage({ params: Promise.resolve({ id: "cg-99" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("redirects to /cardgroups on UNAUTHENTICATED gqlFetch error", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(new Error("UNAUTHENTICATED"));

    await expect(CardsPage({ params: Promise.resolve({ id: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("renders heading and passes initialEdges to CardsClient", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ cardgroup: CARDGROUP } as never)
      .mockResolvedValueOnce(CONNECTION as never);

    const jsx = await CardsPage({ params: Promise.resolve({ id: "cg-1" }) });
    render(jsx);

    expect(screen.getByText("Cards in Vocab")).toBeInTheDocument();
    expect(screen.getByTestId("cards-client")).toHaveAttribute("data-cardgroup-id", "cg-1");
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("World")).toBeInTheDocument();

    const addCardLink = screen.getByRole("link", { name: /\+ add card/i });
    expect(addCardLink).toHaveAttribute(
      "href",
      "/cards/new?cardgroup=cg-1&return=/cardgroups/cg-1/cards",
    );
  });

  it("renders back link to the cardgroups list", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ cardgroup: CARDGROUP } as never)
      .mockResolvedValueOnce(EMPTY_CONNECTION as never);

    const jsx = await CardsPage({ params: Promise.resolve({ id: "cg-1" }) });
    render(jsx);

    const backLink = screen.getByRole("link", { name: /back/i });
    expect(backLink).toHaveAttribute("href", "/cardgroups");
  });
});

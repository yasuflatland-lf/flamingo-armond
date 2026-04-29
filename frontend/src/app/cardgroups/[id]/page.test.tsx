// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Mock next/navigation before importing the page
vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

// Mock createSupabaseServerClient
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

// Mock gqlFetch
vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import CardgroupDetailPage from "./page";

const FIXED_DATE = "2024-06-15T10:00:00.000Z";

function makeSupabaseMock(user: { id: string } | null) {
  return {
    auth: {
      getUser: vi.fn().mockResolvedValue({
        data: { user },
        error: null,
      }),
    },
  };
}

function makeParams(id: string) {
  return Promise.resolve({ id });
}

describe("CardgroupDetailPage", () => {
  it("redirects to /login when no user is authenticated", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    await expect(CardgroupDetailPage({ params: makeParams("cg-1") })).rejects.toThrow(
      "REDIRECT:/login",
    );
  });

  it("redirects to /cardgroups when cardgroup is null", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ cardgroup: null } as never)
      .mockResolvedValueOnce({ cardsByCardgroup: [] } as never);

    await expect(CardgroupDetailPage({ params: makeParams("cg-missing") })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("redirects to /cardgroups when gqlFetch throws UNAUTHENTICATED", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error('GraphQL errors: [{"extensions":{"code":"UNAUTHENTICATED"}}]'),
    );

    await expect(CardgroupDetailPage({ params: makeParams("cg-1") })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("happy path: renders name, preview list, and three CTAs with correct hrefs", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Spanish Vocab", updatedAt: FIXED_DATE },
      } as never)
      .mockResolvedValueOnce({
        cardsByCardgroup: [
          {
            id: "card-1",
            front: "Hola",
            back: "Hello",
            due: FIXED_DATE,
            state: 0,
            cardgroupId: "cg-1",
          },
          {
            id: "card-2",
            front: "Adiós",
            back: "Goodbye",
            due: FIXED_DATE,
            state: 0,
            cardgroupId: "cg-1",
          },
        ],
      } as never);

    const jsx = await CardgroupDetailPage({ params: makeParams("cg-1") });
    render(jsx);

    // Heading
    expect(screen.getByRole("heading", { name: "Spanish Vocab" })).toBeInTheDocument();

    // Preview cards
    expect(screen.getByText("Hola")).toBeInTheDocument();
    expect(screen.getByText("Adiós")).toBeInTheDocument();

    // Three CTAs
    const startLink = screen.getByRole("link", { name: /start learning/i });
    expect(startLink).toHaveAttribute("href", "/learn/cg-1");

    const editLink = screen.getByRole("link", { name: /edit/i });
    expect(editLink).toHaveAttribute("href", "/cardgroups/cg-1/edit");

    const manageLink = screen.getByRole("link", { name: /manage cards/i });
    expect(manageLink).toHaveAttribute("href", "/cardgroups/cg-1/cards");
  });

  it("shows 'Showing 5 of N' text when there are more than 5 cards", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );

    const manyCards = Array.from({ length: 8 }, (_, i) => ({
      id: `card-${i}`,
      front: `Front ${i}`,
      back: `Back ${i}`,
      due: FIXED_DATE,
      state: 0,
      cardgroupId: "cg-1",
    }));

    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Big Deck", updatedAt: FIXED_DATE },
      } as never)
      .mockResolvedValueOnce({ cardsByCardgroup: manyCards } as never);

    const jsx = await CardgroupDetailPage({ params: makeParams("cg-1") });
    render(jsx);

    expect(screen.getByText("Showing 5 of 8")).toBeInTheDocument();
    // Only 5 preview cards shown
    expect(screen.getAllByRole("listitem")).toHaveLength(5);
  });

  it("renders empty state with Add card CTA when there are no cards", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );

    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Empty Deck", updatedAt: FIXED_DATE },
      } as never)
      .mockResolvedValueOnce({ cardsByCardgroup: [] } as never);

    const jsx = await CardgroupDetailPage({ params: makeParams("cg-1") });
    render(jsx);

    expect(screen.getByText(/no cards yet/i)).toBeInTheDocument();

    const addCardLink = screen.getByRole("link", { name: /add card/i });
    expect(addCardLink).toHaveAttribute("href", "/cardgroups/cg-1/cards");
  });

  it("truncates card front text longer than 80 characters", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );

    const longFront = "A".repeat(85);

    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Truncation Test", updatedAt: FIXED_DATE },
      } as never)
      .mockResolvedValueOnce({
        cardsByCardgroup: [
          {
            id: "card-1",
            front: longFront,
            back: "Back",
            due: FIXED_DATE,
            state: 0,
            cardgroupId: "cg-1",
          },
        ],
      } as never);

    const jsx = await CardgroupDetailPage({ params: makeParams("cg-1") });
    render(jsx);

    const expectedText = `${"A".repeat(79)}…`;
    expect(screen.getByText(expectedText)).toBeInTheDocument();
  });
});

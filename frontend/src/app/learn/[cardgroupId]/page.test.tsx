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

vi.mock("./learn-client", () => ({
  LearnClient: ({
    cardgroupId,
    initialCards,
    lastViewedCardgroupId,
  }: {
    cardgroupId: string;
    initialCards: unknown[];
    lastViewedCardgroupId: string | null;
  }) => (
    <div data-testid="learn-client">
      {cardgroupId}:{initialCards.length}:{lastViewedCardgroupId ?? "null"}
    </div>
  ),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import LearnPage from "./page";

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

describe("LearnPage", () => {
  it("redirects to /login when no user is authenticated", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    await expect(LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/login",
    );
  });

  it("redirects to /cardgroups when ownership check fails", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups",
    );
  });

  it("server-renders the first card batch into LearnClient", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Spanish", updatedAt: "2026-04-30T00:00:00Z" },
      } as never)
      .mockResolvedValueOnce({
        learnNextDueCards: [
          {
            id: "c-1",
            front: "Hello",
            back: "Hola",
            userCardState: {
              due: "2026-04-30T00:00:00Z",
              state: 0,
            },
            cardgroupId: "cg-1",
          },
        ],
      } as never)
      .mockResolvedValueOnce({
        me: { id: "user-1", lastViewedCardgroup: { id: "cg-old" } },
        myCardgroups: [],
      } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    render(jsx);

    // Format: cardgroupId:initialCards.length:lastViewedCardgroupId
    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:1:cg-old");
  });

  it("server-renders an empty due batch into LearnClient", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({
        cardgroup: { id: "cg-1", name: "Spanish", updatedAt: "2026-04-30T00:00:00Z" },
      } as never)
      .mockResolvedValueOnce({
        learnNextDueCards: [],
      } as never)
      .mockResolvedValueOnce({
        me: { id: "user-1", lastViewedCardgroup: null },
        myCardgroups: [],
      } as never);

    const jsx = await LearnPage({ params: Promise.resolve({ cardgroupId: "cg-1" }) });
    render(jsx);

    expect(screen.getByTestId("learn-client")).toHaveTextContent("cg-1:0:null");
  });
});

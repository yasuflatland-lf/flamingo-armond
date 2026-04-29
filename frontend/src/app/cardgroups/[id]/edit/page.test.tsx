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

// Stub the client component so the RSC page test does not need Apollo context
vi.mock("./edit-cardgroup-client", () => ({
  EditCardgroupClient: ({ cardgroup }: { cardgroup: { id: string; name: string } }) => (
    <div data-testid="edit-client">{cardgroup.name}</div>
  ),
}));

import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import EditCardgroupPage from "./page";

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
  return { params: Promise.resolve({ id }) };
}

describe("EditCardgroupPage", () => {
  it("redirects to /cardgroups when no user is authenticated", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    await expect(EditCardgroupPage(makeParams("cg-1"))).rejects.toThrow("REDIRECT:/cardgroups");
  });

  it("redirects to /cardgroups when cardgroup is null", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({ cardgroup: null } as never);

    await expect(EditCardgroupPage(makeParams("cg-1"))).rejects.toThrow("REDIRECT:/cardgroups");
  });

  it("redirects to /cardgroups on UNAUTHENTICATED gqlFetch error", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(new Error("GraphQL errors: UNAUTHENTICATED"));

    await expect(EditCardgroupPage(makeParams("cg-1"))).rejects.toThrow("REDIRECT:/cardgroups");
  });

  it("renders the edit client with the cardgroup data", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({
      cardgroup: {
        id: "cg-1",
        name: "Spanish Vocab",
        updatedAt: "2024-06-15T10:00:00.000Z",
      },
    } as never);

    const jsx = await EditCardgroupPage(makeParams("cg-1"));
    render(jsx);

    expect(screen.getByTestId("edit-client")).toBeInTheDocument();
    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
  });
});

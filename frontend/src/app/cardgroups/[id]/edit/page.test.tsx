// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Stub the client component so the RSC page test does not need Apollo context
vi.mock("./cardgroup-management-client", () => ({
  CardgroupManagementClient: ({
    cardgroup,
    initialTotalCount,
  }: {
    cardgroup: { id: string; name: string };
    initialTotalCount: number;
  }) => (
    <div data-testid="management-client">
      <span data-testid="management-name">{cardgroup.name}</span>
      <span data-testid="management-total">{initialTotalCount}</span>
    </div>
  ),
}));

import { headers } from "next/headers";
import { gqlFetch } from "@/lib/apollo/server";
import EditCardgroupPage from "./page";

function makeParams(id: string) {
  return { params: Promise.resolve({ id }) };
}

const cardgroupResult = {
  cardgroup: {
    id: "cg-1",
    name: "Spanish Vocab",
    updatedAt: "2024-06-15T10:00:00.000Z",
  },
};

const connectionResult = {
  cardsByCardgroupConnection: {
    edges: [],
    pageInfo: {
      hasNextPage: false,
      hasPreviousPage: false,
      startCursor: null,
      endCursor: null,
    },
    totalCount: 7,
  },
};

describe("EditCardgroupPage", () => {
  it("redirects to /login when unauthenticated", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-auth-status": "anonymous" }) as never);

    await expect(EditCardgroupPage(makeParams("cg-1"))).rejects.toThrow("REDIRECT:/login");
  });

  it("redirects to /cardgroups when cardgroup is null", async () => {
    vi.mocked(headers).mockResolvedValue(
      new Headers({ "x-auth-status": "authenticated" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce({ cardgroup: null } as never)
      .mockResolvedValueOnce(connectionResult as never);

    await expect(EditCardgroupPage(makeParams("cg-1"))).rejects.toThrow("REDIRECT:/cardgroups");
  });

  it("redirects to /login on UNAUTHENTICATED gqlFetch error", async () => {
    vi.mocked(headers).mockResolvedValue(
      new Headers({ "x-auth-status": "authenticated" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(
      new Error('GraphQL errors: [{"extensions":{"code":"UNAUTHENTICATED"}}]'),
    );

    await expect(EditCardgroupPage(makeParams("cg-1"))).rejects.toThrow("REDIRECT:/login");
  });

  it("renders the management client with cardgroup data and initial connection counts", async () => {
    vi.mocked(headers).mockResolvedValue(
      new Headers({ "x-auth-status": "authenticated" }) as never,
    );
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(cardgroupResult as never)
      .mockResolvedValueOnce(connectionResult as never);

    const jsx = await EditCardgroupPage(makeParams("cg-1"));
    render(jsx);

    expect(screen.getByTestId("management-client")).toBeInTheDocument();
    expect(screen.getByTestId("management-name")).toHaveTextContent("Spanish Vocab");
    expect(screen.getByTestId("management-total")).toHaveTextContent("7");
  });
});

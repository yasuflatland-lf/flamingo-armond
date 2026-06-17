// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
  notFound: vi.fn(() => {
    throw new Error("NOT_FOUND");
  }),
}));
vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));
vi.mock("@/lib/apollo/server", () => ({ gqlFetch: vi.fn() }));
vi.mock("./master-management-client", () => ({
  MasterManagementClient: ({ master }: { master: { name: string } }) => (
    <div data-testid="master-management-client">{master.name}</div>
  ),
}));

import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { gqlFetch } from "@/lib/apollo/server";
import EditMasterPage from "./page";

const ID = "m-1";
const params = () => Promise.resolve({ id: ID });

afterEach(() => vi.clearAllMocks());

describe("EditMasterPage — gate", () => {
  it("redirects to / when not authenticated", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));
    await expect(EditMasterPage({ params: params() })).rejects.toThrow("REDIRECT:/");
    expect(redirect).toHaveBeenCalledWith("/");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  it("calls notFound when adminMaster is null", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({ adminMaster: null } as never);
    await expect(EditMasterPage({ params: params() })).rejects.toThrow("NOT_FOUND");
    expect(notFound).toHaveBeenCalled();
  });

  it("renders the management client when the master is found", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      adminMaster: {
        __typename: "MasterCardgroup",
        id: ID,
        name: "Spanish A1",
        status: "DRAFT",
        cardCount: 3,
      },
    } as never);
    const jsx = await EditMasterPage({ params: params() });
    render(jsx as React.ReactElement);
    expect(screen.getByTestId("master-management-client")).toHaveTextContent("Spanish A1");
  });
});

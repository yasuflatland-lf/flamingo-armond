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

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { NextIntlClientProvider } from "next-intl";
import EditMasterPage from "@/app/admin/masters/[id]/edit/page";
import { gqlFetch } from "@/lib/apollo/server";
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

async function renderPage() {
  const jsx = await EditMasterPage({ params: Promise.resolve({ id: ID }) });
  render(
    <MockedProvider mocks={[]}>
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        {jsx as React.ReactElement}
      </NextIntlClientProvider>
    </MockedProvider>,
  );
}

beforeEach(() => vi.clearAllMocks());
afterEach(() => vi.clearAllMocks());

describe("EditMasterPage — broad integration", () => {
  it("renders the deck name as the h1 and mounts the cards-section placeholder", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({ adminMaster: DECK } as never);
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

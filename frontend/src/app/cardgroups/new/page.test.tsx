// @vitest-environment jsdom

import { InMemoryCache } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";
import { CreateCardgroupDocument, MyCardgroupsDocument } from "@/generated/graphql";
import { NewCardgroupClient } from "./new-cardgroup-client";

// Stub next/navigation so the client component can render outside Next.js.
const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

// Stub next/link so it renders an anchor without Next.js router context.
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const CREATED_CARDGROUP = {
  __typename: "Cardgroup" as const,
  id: "cg-42",
  name: "My New Group",
  updatedAt: "2026-04-30T00:00:00Z",
};

function makeCreateMock(name: string, onCalled?: () => void) {
  return {
    request: {
      query: CreateCardgroupDocument,
      variables: { input: { name } },
    },
    result: () => {
      onCalled?.();
      return {
        data: {
          createCardgroup: {
            __typename: "CreateCardgroupPayload" as const,
            cardgroup: { ...CREATED_CARDGROUP, name },
          },
        },
      };
    },
  };
}

function renderPage(
  mocks: MockedResponse[] = [],
  errorPolicy?: "all" | "none" | "ignore",
  cache?: InMemoryCache,
) {
  const defaultOptions = errorPolicy ? { mutate: { errorPolicy } } : undefined;
  render(
    <MockedProvider mocks={mocks} defaultOptions={defaultOptions} cache={cache}>
      <NewCardgroupClient />
    </MockedProvider>,
  );
}

describe("<NewCardgroupPage> (client)", () => {
  it("renders form with empty name by default", () => {
    renderPage();
    expect(screen.getByRole("textbox")).toHaveValue("");
    expect(screen.getByRole("button", { name: /create/i })).toBeInTheDocument();
  });

  it("renders page header and back link", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: /new cardgroup/i })).toBeInTheDocument();
    const backLink = screen.getByRole("link");
    expect(backLink).toHaveAttribute("href", "/cardgroups");
  });

  it("submitting a valid name triggers the CreateCardgroup mutation", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    renderPage([makeCreateMock("My New Group", mutationCalled)]);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("on success calls router.push with returned id and router.refresh", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    renderPage([makeCreateMock("My New Group")]);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("on success writes the new cardgroup into the MyCardgroups cache", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();

    // Use a real InMemoryCache so the update(cache, { data }) block in
    // new-cardgroup-client.tsx can perform a readQuery/writeQuery round-trip.
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: MyCardgroupsDocument,
      data: { myCardgroups: [] },
    });

    renderPage([makeCreateMock("My New Group")], undefined, cache);

    await user.type(screen.getByRole("textbox"), "My New Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    // Wait for the mutation to complete (navigation is the observable signal).
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CREATED_CARDGROUP.id}`);
    });

    // The cache update callback must have prepended the new cardgroup.
    const result = cache.readQuery({ query: MyCardgroupsDocument });
    expect(result?.myCardgroups).toHaveLength(1);
    expect(result?.myCardgroups[0]).toMatchObject({
      id: CREATED_CARDGROUP.id,
      name: CREATED_CARDGROUP.name,
      updatedAt: CREATED_CARDGROUP.updatedAt,
    });
  });

  it("BAD_USER_INPUT on field 'name' shows inline error", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "Bad Name" } },
        },
        result: {
          errors: [
            new GraphQLError("name already exists", {
              extensions: { code: "BAD_USER_INPUT", field: "name" },
            }),
          ],
        },
      },
    ];

    renderPage(mocks, "all");

    await user.type(screen.getByRole("textbox"), "Bad Name");
    await user.click(screen.getByRole("button", { name: /create/i }));

    const errorEl = await screen.findByText("name already exists");
    expect(errorEl).toBeInTheDocument();
    expect(errorEl.className).toMatch(/text-destructive/);
  });

  it("UNAUTHENTICATED error shows session-expired banner", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: CreateCardgroupDocument,
          variables: { input: { name: "My Group" } },
        },
        result: {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      },
    ];

    renderPage(mocks, "all");

    await user.type(screen.getByRole("textbox"), "My Group");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

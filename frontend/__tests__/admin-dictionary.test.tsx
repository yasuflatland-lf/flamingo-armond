// @vitest-environment jsdom
/**
 * Broad page-level tests for /admin/dictionary.
 *
 * Coverage in this file:
 *   - RSC auth gate: logged-out → redirect /login
 *   - RSC auth gate: Supabase transport error → rethrow
 *   - Client mount with cardgroups: picker is pre-populated
 *   - Client mount with no cardgroups: picker shows only the default option
 *   - Initial form state (untouched): Import button is disabled before Validate runs
 *
 * NOT covered here (owned by admin-dictionary-import.test.tsx):
 *   - validate → preview → import flow
 *   - FORBIDDEN handling at the mutation level
 *   - stale-validation invalidation after textarea edit
 *
 * NOT covered here (owned by admin-layout.test.tsx):
 *   - Admin sidebar rendering and route highlighting
 *   - AdminLayout admin-role gate
 */

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ---------------------------------------------------------------------------
// Imports — after vi.mock declarations
// ---------------------------------------------------------------------------

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { redirect } from "next/navigation";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DictionaryImportClient } from "@/app/admin/dictionary/dictionary-client";
import AdminDictionaryPage from "@/app/admin/dictionary/page";
import { MyCardgroupsDocument } from "@/generated/graphql";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "./utils/mock-supabase";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CARDGROUPS_MOCK = {
  request: {
    query: MyCardgroupsDocument,
    variables: {},
  },
  result: {
    data: {
      myCardgroups: [
        {
          __typename: "Cardgroup",
          id: "cg-100",
          name: "Vocab Set A",
          updatedAt: "2026-01-01T00:00:00Z",
        },
        {
          __typename: "Cardgroup",
          id: "cg-200",
          name: "Grammar Notes",
          updatedAt: "2026-01-02T00:00:00Z",
        },
      ],
    },
  },
};

const EMPTY_CARDGROUPS_MOCK = {
  request: {
    query: MyCardgroupsDocument,
    variables: {},
  },
  result: {
    data: {
      myCardgroups: [],
    },
  },
};

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  resetMockSupabase();
  vi.clearAllMocks();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
});

afterEach(() => {
  consoleErrorSpy.mockRestore();
  consoleWarnSpy.mockRestore();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// RSC auth gate
// ---------------------------------------------------------------------------

describe("AdminDictionaryPage (RSC auth gate)", () => {
  it("redirects to /login when no user is signed in", async () => {
    setMockSupabaseUser(null);

    await expect(AdminDictionaryPage()).rejects.toThrow("REDIRECT:/login");

    expect(redirect).toHaveBeenCalledWith("/login");
  });

  it("rethrows a Supabase transport error without redirecting", async () => {
    const transportErr = new Error("supabase network failure");
    setMockSupabaseUserError(transportErr);

    await expect(AdminDictionaryPage()).rejects.toBe(transportErr);

    expect(redirect).not.toHaveBeenCalled();
  });

  it("renders DictionaryImportClient when the user is authenticated", async () => {
    setMockSupabaseUser({ id: "u-authenticated" });

    const tree = await AdminDictionaryPage();

    // The RSC returns a React element — confirm it is non-null and not a redirect.
    expect(tree).not.toBeNull();
    expect(redirect).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Client mount: cardgroup picker
// ---------------------------------------------------------------------------

describe("DictionaryImportClient (page-level integration)", () => {
  it("pre-populates the cardgroup picker with names returned by the query", async () => {
    render(
      <MockedProvider mocks={[CARDGROUPS_MOCK]}>
        <DictionaryImportClient />
      </MockedProvider>,
    );

    // Both cardgroup names must appear in the select options.
    const option1 = await screen.findByRole("option", { name: "Vocab Set A" });
    const option2 = screen.getByRole("option", { name: "Grammar Notes" });

    expect(option1).toBeInTheDocument();
    expect(option2).toBeInTheDocument();
  });

  it("shows only the default placeholder option when no cardgroups exist", async () => {
    render(
      <MockedProvider mocks={[EMPTY_CARDGROUPS_MOCK]}>
        <DictionaryImportClient />
      </MockedProvider>,
    );

    // The default "Select a cardgroup" option must appear.
    const placeholder = await screen.findByRole("option", { name: /select a cardgroup/i });
    expect(placeholder).toBeInTheDocument();

    // No other options should exist beyond the placeholder.
    const select = screen.getByRole("combobox", { name: /target cardgroup/i });
    expect(select.querySelectorAll("option")).toHaveLength(1);
  });

  it("disables the Import button on initial mount before any validation", async () => {
    render(
      <MockedProvider mocks={[CARDGROUPS_MOCK]}>
        <DictionaryImportClient />
      </MockedProvider>,
    );

    // Wait for the cardgroups query to complete so the picker is stable.
    await screen.findByRole("option", { name: "Vocab Set A" });

    // Import button must be disabled: no validation has run, canImport is false.
    const importBtn = screen.getByRole("button", { name: /^import$/i });
    expect(importBtn).toBeDisabled();
  });

  it("disables the Validate button when the payload textarea is empty", async () => {
    render(
      <MockedProvider mocks={[CARDGROUPS_MOCK]}>
        <DictionaryImportClient />
      </MockedProvider>,
    );

    // Wait for stable render.
    await screen.findByRole("option", { name: "Vocab Set A" });

    // Validate button must be disabled when the textarea is empty.
    const validateBtn = screen.getByRole("button", { name: /^validate$/i });
    expect(validateBtn).toBeDisabled();
  });
});

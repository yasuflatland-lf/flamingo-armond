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
import CardgroupsPage from "./page";

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

describe("CardgroupsPage", () => {
  it("redirects to /login when no user is authenticated", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");
  });

  it("renders empty state when myCardgroups is empty", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({ myCardgroups: [] } as never);

    const jsx = await CardgroupsPage();
    render(jsx);

    expect(screen.getByText("You haven't created any cardgroups yet.")).toBeInTheDocument();
    // Empty state CTA link should be present
    const ctaLink = screen.getByRole("link", { name: /new cardgroup/i });
    expect(ctaLink).toBeInTheDocument();
    expect(ctaLink).toHaveAttribute("href", "/cardgroups/new");
  });

  it("does not render footer link when cardgroups list is empty", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({ myCardgroups: [] } as never);

    const jsx = await CardgroupsPage();
    render(jsx);

    // Only one "New cardgroup" link: the empty-state CTA (no footer link when empty)
    const links = screen.getAllByRole("link", { name: /new cardgroup/i });
    expect(links).toHaveLength(1);
    // The single link is the empty-state CTA, not a footer-style link
    expect(links[0]).toHaveAttribute("href", "/cardgroups/new");
  });

  it("renders one list item per cardgroup", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({
      myCardgroups: [
        { id: "cg-1", name: "Spanish Vocab", updatedAt: "2024-06-15T10:00:00.000Z" },
        { id: "cg-2", name: "Math Formulas", updatedAt: "2024-05-20T08:00:00.000Z" },
      ],
    } as never);

    const jsx = await CardgroupsPage();
    render(jsx);

    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();

    const spanishLink = screen.getByRole("link", { name: /spanish vocab/i });
    expect(spanishLink).toHaveAttribute("href", "/cardgroups/cg-1");

    const mathLink = screen.getByRole("link", { name: /math formulas/i });
    expect(mathLink).toHaveAttribute("href", "/cardgroups/cg-2");
  });

  it("renders footer-style New cardgroup link when cardgroups list is non-empty", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({
      myCardgroups: [
        { id: "cg-1", name: "Spanish Vocab", updatedAt: "2024-06-15T10:00:00.000Z" },
      ],
    } as never);

    const jsx = await CardgroupsPage();
    render(jsx);

    // Footer link should be present
    const footerLink = screen.getByRole("link", { name: /new cardgroup/i });
    expect(footerLink).toBeInTheDocument();
    expect(footerLink).toHaveAttribute("href", "/cardgroups/new");
  });

  it("does not render the top-right New cardgroup button when cardgroups exist", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockResolvedValue({
      myCardgroups: [
        { id: "cg-1", name: "Spanish Vocab", updatedAt: "2024-06-15T10:00:00.000Z" },
      ],
    } as never);

    const jsx = await CardgroupsPage();
    render(jsx);

    // Exactly one "New cardgroup" link: the footer link only (no top-right button)
    const links = screen.getAllByRole("link", { name: /new cardgroup/i });
    expect(links).toHaveLength(1);
  });

  it("redirects to /login when MyCardgroupsQuery returns UNAUTHENTICATED", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(new Error("GraphQL errors: UNAUTHENTICATED"));

    await expect(CardgroupsPage()).rejects.toThrow("REDIRECT:/login");
  });

  it("rethrows non-auth errors so the error boundary handles them", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(
      makeSupabaseMock({ id: "user-1" }) as never,
    );
    vi.mocked(gqlFetch).mockRejectedValue(new Error("Network unreachable"));

    await expect(CardgroupsPage()).rejects.toThrow("Network unreachable");
  });
});

// @vitest-environment node
/**
 * CardsPage now redirects to /cardgroups/:id/edit (the integrated management
 * screen). The broad integration test for the cardgroup management screen
 * lives in __tests__/cardgroup-edit.test.tsx. This file remains only to assert
 * that the legacy URL still routes users to the canonical destination.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

import CardsPage from "@/app/cardgroups/[id]/cards/page";

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("CardsPage — legacy URL redirect", () => {
  it("redirects /cardgroups/:id/cards to /cardgroups/:id/edit", async () => {
    await expect(CardsPage({ params: Promise.resolve({ id: "cardgroup-001" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups/cardgroup-001/edit",
    );
  });

  it("preserves the id segment when redirecting (UUID-style id)", async () => {
    const id = "0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c";
    await expect(CardsPage({ params: Promise.resolve({ id }) })).rejects.toThrow(
      `REDIRECT:/cardgroups/${id}/edit`,
    );
  });
});

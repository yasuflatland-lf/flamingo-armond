// @vitest-environment node
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

import CardsPage from "./page";

describe("CardsPage (RSC)", () => {
  it("redirects to /cardgroups/:id/edit (canonical management screen)", async () => {
    await expect(CardsPage({ params: Promise.resolve({ id: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups/cg-1/edit",
    );
  });

  it("preserves the id segment when redirecting (UUID-style id)", async () => {
    await expect(
      CardsPage({ params: Promise.resolve({ id: "0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c" }) }),
    ).rejects.toThrow("REDIRECT:/cardgroups/0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c/edit");
  });
});

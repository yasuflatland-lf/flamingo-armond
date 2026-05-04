// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

import { redirect } from "next/navigation";
import CardgroupDetailPage from "./page";

describe("CardgroupDetailPage (redirect-only)", () => {
  it("redirects to /cardgroups/<id>/cards for a plain id", async () => {
    await expect(CardgroupDetailPage({ params: Promise.resolve({ id: "cg-1" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups/cg-1/cards",
    );
    expect(redirect).toHaveBeenCalledWith("/cardgroups/cg-1/cards");
  });

  it("URL-encodes the id segment so reserved characters do not break the destination", async () => {
    await expect(CardgroupDetailPage({ params: Promise.resolve({ id: "a/b?c" }) })).rejects.toThrow(
      "REDIRECT:/cardgroups/a%2Fb%3Fc/cards",
    );
  });
});

// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// CardgroupDetailPage now redirects to /cardgroups/:id/edit (the integrated
// management screen). The page no longer fetches data, performs auth gating,
// or renders any UI — all of those responsibilities have moved to
// /cardgroups/[id]/edit/page.tsx and are covered by its broad integration
// tests there.

const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

import CardgroupDetailPage from "@/app/cardgroups/[id]/page";

function makeParams(id: string): Promise<{ id: string }> {
  return Promise.resolve({ id });
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("CardgroupDetailPage (broad page-level)", () => {
  it("redirects to /cardgroups/:id/edit (canonical management screen)", async () => {
    await expect(CardgroupDetailPage({ params: makeParams("cg-1") })).rejects.toThrow(
      `${REDIRECT_PREFIX}/cardgroups/cg-1/edit`,
    );
  });

  it("preserves the id segment when redirecting (UUID-style id)", async () => {
    const id = "0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c";
    await expect(CardgroupDetailPage({ params: makeParams(id) })).rejects.toThrow(
      `${REDIRECT_PREFIX}/cardgroups/${id}/edit`,
    );
  });
});

// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type AddCardDetail,
  type AddMasterCardDetail,
  dispatchFlamingo,
  FLAMINGO_EVENT,
  type SearchStateDetail,
  subscribeFlamingo,
} from "./flamingo-events";

// Listeners registered per-test, torn down in afterEach so a failing assertion
// never leaks a handler into the next test.
const registered: Array<[string, EventListener]> = [];
function listen(type: string, handler: EventListener) {
  window.addEventListener(type, handler);
  registered.push([type, handler]);
}
afterEach(() => {
  for (const [type, handler] of registered.splice(0)) {
    window.removeEventListener(type, handler);
  }
});

describe("FLAMINGO_EVENT", () => {
  it("keeps the exact wire string values (rename regression guard)", () => {
    // These strings are the shell <-> page contract. A rename here silently
    // breaks every dispatcher/listener that was not updated in lockstep.
    expect(FLAMINGO_EVENT).toEqual({
      openSearch: "flamingo:open-search",
      searchState: "flamingo:search-state",
      addCardgroup: "flamingo:add-cardgroup",
      addCard: "flamingo:add-card",
      addMasterCard: "flamingo:add-master-card",
      batchImport: "flamingo:batch-import",
      merge: "flamingo:merge",
    });
  });
});

describe("dispatchFlamingo", () => {
  it("delivers the search-state detail to a window listener", () => {
    const received: SearchStateDetail[] = [];
    listen(FLAMINGO_EVENT.searchState, (e) => {
      received.push((e as CustomEvent<SearchStateDetail>).detail);
    });
    dispatchFlamingo(FLAMINGO_EVENT.searchState, { detail: { active: true, visible: false } });
    expect(received).toEqual([{ active: true, visible: false }]);
  });

  it("delivers the add-card detail (cardgroupId)", () => {
    const received: AddCardDetail[] = [];
    listen(FLAMINGO_EVENT.addCard, (e) => {
      received.push((e as CustomEvent<AddCardDetail>).detail);
    });
    dispatchFlamingo(FLAMINGO_EVENT.addCard, { detail: { cardgroupId: "g1" }, cancelable: true });
    expect(received).toEqual([{ cardgroupId: "g1" }]);
  });

  it("delivers the add-master-card detail (masterId)", () => {
    const received: AddMasterCardDetail[] = [];
    listen(FLAMINGO_EVENT.addMasterCard, (e) => {
      received.push((e as CustomEvent<AddMasterCardDetail>).detail);
    });
    dispatchFlamingo(FLAMINGO_EVENT.addMasterCard, {
      detail: { masterId: "m1" },
      cancelable: true,
    });
    expect(received).toEqual([{ masterId: "m1" }]);
  });

  it("dispatches a no-detail event (open-search)", () => {
    let fired = false;
    listen(FLAMINGO_EVENT.openSearch, () => {
      fired = true;
    });
    dispatchFlamingo(FLAMINGO_EVENT.openSearch);
    expect(fired).toBe(true);
  });

  it("returns true when no listener cancels a cancelable event", () => {
    expect(
      dispatchFlamingo(FLAMINGO_EVENT.addCard, {
        detail: { cardgroupId: "g1" },
        cancelable: true,
      }),
    ).toBe(true);
  });

  it("returns false when a listener calls preventDefault (router-push fallback gate)", () => {
    listen(FLAMINGO_EVENT.addCard, (e) => e.preventDefault());
    expect(
      dispatchFlamingo(FLAMINGO_EVENT.addCard, {
        detail: { cardgroupId: "g1" },
        cancelable: true,
      }),
    ).toBe(false);
  });
});

describe("batch-import + merge events", () => {
  it("round-trips a DeckOwnerDetail payload for batch-import", () => {
    const handler = vi.fn();
    const off = subscribeFlamingo(FLAMINGO_EVENT.batchImport, handler);
    dispatchFlamingo(FLAMINGO_EVENT.batchImport, { detail: { ownerId: "deck-1" } });
    off();
    expect(handler).toHaveBeenCalledTimes(1);
    expect(handler).toHaveBeenCalledWith({ ownerId: "deck-1" }, expect.any(CustomEvent));
  });

  it("round-trips a DeckOwnerDetail payload for merge", () => {
    const handler = vi.fn();
    const off = subscribeFlamingo(FLAMINGO_EVENT.merge, handler);
    dispatchFlamingo(FLAMINGO_EVENT.merge, { detail: { ownerId: "deck-2" } });
    off();
    expect(handler).toHaveBeenCalledWith({ ownerId: "deck-2" }, expect.any(CustomEvent));
  });
});

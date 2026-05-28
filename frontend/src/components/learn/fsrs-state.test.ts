import { describe, expect, it } from "vitest";
import { masteryStage } from "./fsrs-state";

describe("masteryStage — state to stage mapping", () => {
  it("maps state 0 to new", () => {
    expect(masteryStage(0)).toBe("new");
  });

  it("maps state 1 to learning", () => {
    expect(masteryStage(1)).toBe("learning");
  });

  it("maps state 2 to learned", () => {
    expect(masteryStage(2)).toBe("learned");
  });

  it("maps state 3 to learning", () => {
    expect(masteryStage(3)).toBe("learning");
  });

  it("falls back to new for unknown states (99)", () => {
    expect(masteryStage(99)).toBe("new");
  });

  it("falls back to new for negative states (-1)", () => {
    expect(masteryStage(-1)).toBe("new");
  });
});

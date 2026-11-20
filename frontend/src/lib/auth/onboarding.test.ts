import { describe, expect, it } from "vitest";
import { isUserOnboarded } from "./onboarding";

describe("isUserOnboarded", () => {
  it("returns true when displayName is a non-empty string", () => {
    expect(isUserOnboarded({ displayName: "Alice" })).toBe(true);
  });

  it("returns false when displayName is null", () => {
    expect(isUserOnboarded({ displayName: null })).toBe(false);
  });

  it("returns false when displayName is undefined", () => {
    expect(isUserOnboarded({ displayName: undefined })).toBe(false);
  });

  it("returns false when displayName is an empty string", () => {
    expect(isUserOnboarded({ displayName: "" })).toBe(false);
  });

  it("returns false when displayName is whitespace only", () => {
    expect(isUserOnboarded({ displayName: "   " })).toBe(false);
  });

  it("returns true when displayName is only U+FEFF, which the backend accepts", () => {
    expect(isUserOnboarded({ displayName: "\uFEFF" })).toBe(true);
  });

  it("returns false when displayName is only U+0085, which the backend rejects", () => {
    expect(isUserOnboarded({ displayName: "\u0085" })).toBe(false);
  });

  it("returns false when me is null", () => {
    expect(isUserOnboarded(null)).toBe(false);
  });

  it("returns false when me is undefined", () => {
    expect(isUserOnboarded(undefined)).toBe(false);
  });
});

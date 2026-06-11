import { afterEach, describe, expect, it, vi } from "vitest";
import enMessages from "../../messages/en.json";
import jaMessages from "../../messages/ja.json";
import type { Locale } from "./config";
import { loadMessages } from "./load-messages";

describe("loadMessages", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the requested locale catalog", async () => {
    // Compare against the imported catalog rather than an inline literal, so the
    // assertion carries no non-English copy (language-policy.md).
    expect(await loadMessages("ja")).toEqual(jaMessages);
  });

  it("falls back to the default catalog and logs when the requested catalog is missing", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    // A locale with no committed catalog — simulates "added to `locales`, forgot the json".
    const messages = await loadMessages("xx" as Locale);

    // Falls back to the default (en) catalog, which is always present.
    expect(messages).toEqual(enMessages);
    expect(errorSpy).toHaveBeenCalled();
  });
});

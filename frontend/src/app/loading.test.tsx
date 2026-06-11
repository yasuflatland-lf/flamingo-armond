// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import Loading from "./loading";

vi.mock("next-intl/server", () => ({
  getTranslations: vi.fn().mockResolvedValue((key: string) => {
    const messages: Record<string, string> = { loading: "Loading..." };
    return messages[key] ?? key;
  }),
}));

describe("root <Loading>", () => {
  it("renders the branded loading splash with an accessible status region", async () => {
    render(await Loading());
    expect(screen.getByRole("status", { name: /loading/i })).toBeInTheDocument();
  });
});

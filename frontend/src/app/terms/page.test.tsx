// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next-intl/server", () => ({
  getLocale: vi.fn(async () => "en"),
}));

import { getLocale } from "next-intl/server";
import TermsPage, { generateMetadata } from "./page";

describe("TermsPage", () => {
  it("en locale: renders Terms of Service heading", async () => {
    const jsx = await TermsPage();
    render(jsx);

    expect(
      screen.getByRole("heading", { level: 1, name: "Terms of Service" }),
    ).toBeInTheDocument();
  });

  it("en locale: renders all 11 section headings", async () => {
    const jsx = await TermsPage();
    render(jsx);

    expect(
      screen.getByRole("heading", { level: 2, name: "1. Acceptance of Terms" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 2, name: "11. Contact" }),
    ).toBeInTheDocument();
  });

  it("en locale: renders back-to-login link pointing to /login", async () => {
    const jsx = await TermsPage();
    render(jsx);

    const link = screen.getByRole("link", { name: /back to login/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", "/login");
  });

  it("ja locale: renders Japanese heading and back link", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");

    const jsx = await TermsPage();
    render(jsx);

    expect(screen.getByRole("heading", { level: 1, name: "利用規約" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /ログインに戻る/ })).toBeInTheDocument();
  });

  it("ja locale: renders Japanese section headings", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");

    const jsx = await TermsPage();
    render(jsx);

    expect(
      screen.getByRole("heading", { level: 2, name: "1. 利用規約への同意" }),
    ).toBeInTheDocument();
  });

  it("generateMetadata en: title is Terms of Service", async () => {
    const meta = await generateMetadata();
    expect(meta.title).toBe("Terms of Service");
  });

  it("generateMetadata ja: title is 利用規約", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");
    const meta = await generateMetadata();
    expect(meta.title).toBe("利用規約");
  });
});

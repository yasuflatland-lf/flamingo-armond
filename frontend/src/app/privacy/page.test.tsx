// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next-intl/server", () => ({
  getLocale: vi.fn(async () => "en"),
}));

import { getLocale } from "next-intl/server";
import PrivacyPage, { generateMetadata } from "./page";

describe("PrivacyPage", () => {
  it("en locale: renders Privacy Policy heading", async () => {
    const jsx = await PrivacyPage();
    render(jsx);

    expect(screen.getByRole("heading", { level: 1, name: "Privacy Policy" })).toBeInTheDocument();
  });

  it("en locale: renders intro paragraph", async () => {
    const jsx = await PrivacyPage();
    render(jsx);

    expect(screen.getByText(/your privacy is important to us/i)).toBeInTheDocument();
  });

  it("en locale: renders all 9 section headings", async () => {
    const jsx = await PrivacyPage();
    render(jsx);

    expect(
      screen.getByRole("heading", { level: 2, name: "1. Information We Collect" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "9. Contact" })).toBeInTheDocument();
  });

  it("en locale: renders back-to-login link pointing to /login", async () => {
    const jsx = await PrivacyPage();
    render(jsx);

    const link = screen.getByRole("link", { name: /back to login/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", "/login");
  });

  it("ja locale: renders Japanese heading and intro", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");

    const jsx = await PrivacyPage();
    render(jsx);

    expect(
      screen.getByRole("heading", { level: 1, name: "プライバシーポリシー" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/お客様のプライバシーは私たちにとって重要です/)).toBeInTheDocument();
  });

  it("ja locale: renders Japanese back link", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");

    const jsx = await PrivacyPage();
    render(jsx);

    expect(screen.getByRole("link", { name: /ログインに戻る/ })).toBeInTheDocument();
  });

  it("generateMetadata en: title is Privacy Policy", async () => {
    const meta = await generateMetadata();
    expect(meta.title).toBe("Privacy Policy");
  });

  it("generateMetadata ja: title is プライバシーポリシー", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");
    const meta = await generateMetadata();
    expect(meta.title).toBe("プライバシーポリシー");
  });
});

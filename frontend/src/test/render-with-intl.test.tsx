// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import { useTranslations } from "next-intl";
import { describe, expect, it } from "vitest";
import { renderWithIntl } from "./render-with-intl";

function Probe() {
  const t = useTranslations("Common");
  return <span>{t("save")}</span>;
}

describe("renderWithIntl", () => {
  it("resolves keys against the en catalog by default", () => {
    renderWithIntl(<Probe />);
    expect(screen.getByText("Save")).toBeInTheDocument();
  });
});

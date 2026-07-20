// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import enMessages from "../../../../../messages/en.json";
import jaMessages from "../../../../../messages/ja.json";
import { CatalogDeckSkeleton } from "./catalog-deck-skeleton";

describe("<CatalogDeckSkeleton>", () => {
  it("labels the placeholder list from the Catalog catalog namespace", () => {
    renderWithIntl(<CatalogDeckSkeleton />);

    expect(screen.getByRole("list", { name: enMessages.Catalog.loadingDeck })).toBeInTheDocument();
  });

  it("localizes the aria-label so assistive tech matches the page locale", () => {
    renderWithIntl(<CatalogDeckSkeleton />, { locale: "ja", messages: jaMessages });

    expect(screen.getByRole("list", { name: jaMessages.Catalog.loadingDeck })).toBeInTheDocument();
  });
});

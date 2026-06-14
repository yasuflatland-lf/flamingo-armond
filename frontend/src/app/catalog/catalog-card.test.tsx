// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { makeFragmentData } from "@/generated/fragment-masking";
import { renderWithIntl } from "@/test/render-with-intl";
import { CatalogCard } from "./catalog-card";

// `makeFragmentData` is identity at runtime, so the wrapped object still carries
// every field the component reads via `useFragment`; the wrap only supplies the
// masked `FragmentType` the `node` prop now expects.
const FULL_NODE = makeFragmentData(
  {
    __typename: "MasterCardgroup" as const,
    id: "m-1",
    name: "Business English",
    description: "Professional vocabulary",
    language: "en",
    level: "B2",
    category: "Business",
    cardCount: 42,
  },
  CatalogCardFieldsFragment,
);

const BARE_NODE = makeFragmentData(
  {
    __typename: "MasterCardgroup" as const,
    id: "m-2",
    name: "JLPT N3 Kanji",
    description: null,
    language: null,
    level: null,
    category: null,
    cardCount: 100,
  },
  CatalogCardFieldsFragment,
);

describe("<CatalogCard>", () => {
  it("renders name, card count, and metadata badges when present", () => {
    renderWithIntl(
      <CatalogCard node={FULL_NODE} importing={false} imported={false} onImport={vi.fn()} />,
    );

    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("Professional vocabulary")).toBeInTheDocument();
    expect(screen.getByText("42 cards")).toBeInTheDocument();
    expect(screen.getByText("en")).toBeInTheDocument();
    expect(screen.getByText("Level B2")).toBeInTheDocument();
    expect(screen.getByText("Business")).toBeInTheDocument();
  });

  it("omits the description and all metadata badges when those fields are null", () => {
    renderWithIntl(
      <CatalogCard node={BARE_NODE} importing={false} imported={false} onImport={vi.fn()} />,
    );

    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.getByText("100 cards")).toBeInTheDocument();
    // No level/category/language badges are rendered for null fields.
    expect(screen.queryByText(/^Level /)).not.toBeInTheDocument();
    expect(screen.queryByText("Business")).not.toBeInTheDocument();
  });

  it("disables the button and shows the in-flight label while importing", () => {
    renderWithIntl(
      <CatalogCard node={FULL_NODE} importing={true} imported={false} onImport={vi.fn()} />,
    );

    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Importing...");
  });

  it("disables the button and shows the imported label once imported", () => {
    renderWithIntl(
      <CatalogCard node={FULL_NODE} importing={false} imported={true} onImport={vi.fn()} />,
    );

    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Imported");
  });

  it("calls onImport with the deck id when the idle button is clicked", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    renderWithIntl(
      <CatalogCard node={FULL_NODE} importing={false} imported={false} onImport={onImport} />,
    );

    await user.click(screen.getByTestId("catalog-import-m-1"));

    expect(onImport).toHaveBeenCalledWith("m-1");
  });

  it("uses custom labels and a custom testId prefix when provided", () => {
    renderWithIntl(
      <CatalogCard
        node={FULL_NODE}
        importing={false}
        imported={false}
        onImport={vi.fn()}
        labels={{ action: "Start with this deck", inProgress: "Starting...", done: "Added" }}
        testIdPrefix="onboarding-deck"
      />,
    );

    const btn = screen.getByTestId("onboarding-deck-m-1");
    expect(btn).toHaveTextContent("Start with this deck");
    // The default catalog testId must NOT be present when a prefix override is given.
    expect(screen.queryByTestId("catalog-import-m-1")).toBeNull();
  });
});

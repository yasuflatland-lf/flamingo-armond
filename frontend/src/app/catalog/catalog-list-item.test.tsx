// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { makeFragmentData } from "@/generated/fragment-masking";
import { renderWithIntl } from "@/test/render-with-intl";
import { CatalogListItem } from "./catalog-list-item";

// `makeFragmentData` is identity at runtime, so the wrapped object still carries
// every field the component reads via `useFragment`; the wrap only supplies the
// masked `FragmentType` the `node` prop expects.
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

function renderItem(
  node: typeof FULL_NODE,
  props: Partial<{ importing: boolean; imported: boolean; onImport: (id: string) => void }> = {},
) {
  const onImport = props.onImport ?? vi.fn();
  renderWithIntl(
    <ul>
      <CatalogListItem
        node={node}
        importing={props.importing ?? false}
        imported={props.imported ?? false}
        onImport={onImport}
      />
    </ul>,
  );
  return { onImport };
}

describe("<CatalogListItem>", () => {
  it("renders name, card count, and metadata badges when present", () => {
    renderItem(FULL_NODE);
    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("42 cards")).toBeInTheDocument();
    expect(screen.getByText("en")).toBeInTheDocument();
    expect(screen.getByText("Level B2")).toBeInTheDocument();
    expect(screen.getByText("Business")).toBeInTheDocument();
  });

  it("does not render the description text", () => {
    renderItem(FULL_NODE);
    expect(screen.queryByText("Professional vocabulary")).not.toBeInTheDocument();
  });

  it("omits all metadata badges when those fields are null", () => {
    renderItem(BARE_NODE);
    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.getByText("100 cards")).toBeInTheDocument();
    expect(screen.queryByText(/^Level /)).not.toBeInTheDocument();
    expect(screen.queryByText("Business")).not.toBeInTheDocument();
    expect(screen.queryByText("en")).not.toBeInTheDocument();
  });

  it("disables the button and shows the in-flight label while importing", () => {
    renderItem(FULL_NODE, { importing: true });
    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Importing...");
  });

  it("disables the button and shows the imported label once imported", () => {
    renderItem(FULL_NODE, { imported: true });
    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Imported");
  });

  it("calls onImport with the deck id when the idle button is clicked", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    renderItem(FULL_NODE, { onImport });
    await user.click(screen.getByTestId("catalog-import-m-1"));
    expect(onImport).toHaveBeenCalledWith("m-1");
  });

  it("uses custom labels and a custom testId prefix when provided", () => {
    renderWithIntl(
      <ul>
        <CatalogListItem
          node={FULL_NODE}
          importing={false}
          imported={false}
          onImport={vi.fn()}
          labels={{ action: "Start with this deck", inProgress: "Starting...", done: "Added" }}
          testIdPrefix="onboarding-deck"
        />
      </ul>,
    );
    const btn = screen.getByTestId("onboarding-deck-m-1");
    expect(btn).toHaveTextContent("Start with this deck");
    expect(screen.queryByTestId("catalog-import-m-1")).toBeNull();
  });

  it("does not call onImport when the button is in the imported state", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    renderItem(FULL_NODE, { imported: true, onImport });
    await user.click(screen.getByTestId("catalog-import-m-1"));
    expect(onImport).not.toHaveBeenCalled();
  });

  it("exposes a locale-independent row testId on the list item", () => {
    renderItem(FULL_NODE);
    expect(screen.getByTestId("catalog-row-m-1")).toBeInTheDocument();
  });
});

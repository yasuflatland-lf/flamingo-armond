// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CatalogDeckFieldsFragment } from "@/app/catalog/queries";
import { makeFragmentData } from "@/generated/fragment-masking";
import { renderWithIntl } from "@/test/render-with-intl";
import { OnboardingDeckTile } from "./onboarding-deck-tile";

// `makeFragmentData` is identity at runtime, so the wrapped object still carries
// every field the component reads via `useFragment`; the wrap only supplies the
// masked `FragmentType` the `node` prop now expects.
const FULL_NODE = makeFragmentData(
  {
    __typename: "MasterCardgroup" as const,
    id: "m-1",
    name: "Business English",
    description: "Professional vocabulary",
    cardCount: 42,
  },
  CatalogDeckFieldsFragment,
);

const BARE_NODE = makeFragmentData(
  {
    __typename: "MasterCardgroup" as const,
    id: "m-2",
    name: "JLPT N3 Kanji",
    description: null,
    cardCount: 100,
  },
  CatalogDeckFieldsFragment,
);

describe("<OnboardingDeckTile>", () => {
  it("renders name, description, and card count", () => {
    renderWithIntl(<OnboardingDeckTile node={FULL_NODE} importing={false} onImport={vi.fn()} />);

    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("Professional vocabulary")).toBeInTheDocument();
    expect(screen.getByText("42 cards")).toBeInTheDocument();
  });

  it("omits the description when it is null", () => {
    renderWithIntl(<OnboardingDeckTile node={BARE_NODE} importing={false} onImport={vi.fn()} />);

    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.getByText("100 cards")).toBeInTheDocument();
  });

  it("disables the button and shows the in-flight label while importing", () => {
    renderWithIntl(<OnboardingDeckTile node={FULL_NODE} importing={true} onImport={vi.fn()} />);

    const btn = screen.getByTestId("onboarding-deck-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Setting up your environment...");
  });

  it("calls onImport with the deck id when the idle button is clicked", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    renderWithIntl(<OnboardingDeckTile node={FULL_NODE} importing={false} onImport={onImport} />);

    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    expect(onImport).toHaveBeenCalledWith("m-1");
  });
});

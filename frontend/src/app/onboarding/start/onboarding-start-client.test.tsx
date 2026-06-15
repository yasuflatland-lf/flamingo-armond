// @vitest-environment jsdom
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import type { ImportMasterOutcome } from "@/app/catalog/use-import-master";
import { makeFragmentData } from "@/generated/fragment-masking";
import { renderWithIntl } from "@/test/render-with-intl";
import { OnboardingStartClient } from "./onboarding-start-client";

const mockPush = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: mockPush }) }));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const mockImport = vi.fn<(id: string) => Promise<ImportMasterOutcome>>();
vi.mock("@/app/catalog/use-import-master", () => ({
  useImportMaster: () => ({ importMasterCardgroup: mockImport, loading: false }),
}));

// Each cardgroup carries a top-level `id` (read for React keys + per-cardgroup `importing`
// state) plus a masked `CatalogCardFields` ref the chooser hands to `CatalogCard`
// — mirroring the `OnboardingStartQuery` node shape. `makeFragmentData` is
// identity at runtime, so the card's `useFragment` still sees every field.
const M1_FIELDS = {
  __typename: "MasterCardgroup" as const,
  id: "m-1",
  name: "Business English",
  description: "Professional vocabulary",
  language: "en",
  level: "B2",
  category: "Business",
  cardCount: 42,
};

const M2_FIELDS = {
  __typename: "MasterCardgroup" as const,
  id: "m-2",
  name: "JLPT N3 Kanji",
  description: null,
  language: "ja",
  level: null,
  category: null,
  cardCount: 100,
};

const CARDGROUPS = [
  { id: M1_FIELDS.id, ...makeFragmentData(M1_FIELDS, CatalogCardFieldsFragment) },
  { id: M2_FIELDS.id, ...makeFragmentData(M2_FIELDS, CatalogCardFieldsFragment) },
];

beforeEach(() => {
  vi.clearAllMocks();
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe("<OnboardingStartClient>", () => {
  it("renders a card per cardgroup and the create-your-own link", () => {
    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);

    expect(screen.getByTestId("onboarding-deck-m-1")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-deck-m-2")).toBeInTheDocument();
    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-create-link")).toHaveAttribute(
      "href",
      "/cardgroups/new?welcome=1",
    );
  });

  it("renders the preset hero: logo, heading, subline, and preset label", () => {
    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Pick your first cardgroup",
    );
    expect(screen.getByText("Pick a preset and start learning right away.")).toBeInTheDocument();
    // The preset panel's section label (rendered as an h2).
    expect(screen.getByRole("heading", { level: 2 })).toHaveTextContent("Presets");
    // The FlamingoMark logo renders a decorative (aria-hidden) <svg>; query the
    // document directly so the assertion does not depend on renderWithIntl's
    // return shape.
    expect(document.querySelector("svg")).not.toBeNull();
  });

  it("imports the chosen cardgroup and navigates to /learn/{id} on success", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({
      status: "success",
      cardgroupId: "cg-9",
      cardgroupName: "Business English",
    });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    expect(mockImport).toHaveBeenCalledWith("m-1");
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/cg-9");
    });
  });

  it("shows a generic error banner and does not navigate on a rejected import", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "rejected" });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    await waitFor(() => {
      expect(screen.getByTestId("onboarding-import-error")).toBeInTheDocument();
    });
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the not-found copy on a not_found outcome", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "not_found" });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    await waitFor(() => {
      expect(screen.getByTestId("onboarding-import-error")).toHaveTextContent(
        /no longer available/i,
      );
    });
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the auth banner with a sign-in link on an auth outcome", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "auth", kind: "unauthenticated" });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    const banner = await screen.findByTestId("onboarding-import-auth-error");
    expect(banner.querySelector("a")).toHaveAttribute("href", "/login");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the auth banner with a sign-in link on a forbidden outcome", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "auth", kind: "forbidden" });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    const banner = await screen.findByTestId("onboarding-import-auth-error");
    expect(banner.querySelector("a")).toHaveAttribute("href", "/login");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("serializes imports — a second click while one is in flight is ignored", async () => {
    const user = userEvent.setup();
    let resolve: ((o: ImportMasterOutcome) => void) | undefined;
    mockImport.mockReturnValueOnce(
      new Promise<ImportMasterOutcome>((r) => {
        resolve = r;
      }),
    );

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));
    await user.click(screen.getByTestId("onboarding-deck-m-2"));

    expect(mockImport).toHaveBeenCalledTimes(1);

    resolve?.({ status: "success", cardgroupId: "cg-9", cardgroupName: "x" });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/cg-9");
    });
  });
});

// @vitest-environment happy-dom
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CatalogDeckFieldsFragment } from "@/app/catalog/queries";
import type { ImportMasterOutcome } from "@/app/catalog/use-import-master";
import { makeFragmentData } from "@/generated/fragment-masking";
import { renderWithIntl } from "@/test/render-with-intl";
import { OnboardingStartClient } from "./onboarding-start-client";
import type { SeedDefaultStartersOutcome } from "./use-seed-default-starters";

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

const mockSeed = vi.fn<() => Promise<SeedDefaultStartersOutcome>>();
vi.mock("./use-seed-default-starters", () => ({
  useSeedDefaultStarters: () => ({ seedDefaultStarters: mockSeed, loading: false }),
}));

// Each cardgroup carries a top-level `id` (read for React keys + per-cardgroup `importing`
// state) plus a masked `CatalogDeckFields` ref the chooser hands to `CatalogDeckTile`
// — mirroring the `OnboardingStartQuery` node shape. `makeFragmentData` is
// identity at runtime, so the card's `useFragment` still sees every field.
const M1_FIELDS = {
  __typename: "MasterCardgroup" as const,
  id: "m-1",
  name: "Business English",
  description: "Professional vocabulary",
  cardCount: 42,
};

const M2_FIELDS = {
  __typename: "MasterCardgroup" as const,
  id: "m-2",
  name: "JLPT N3 Kanji",
  description: null,
  cardCount: 100,
};

const CARDGROUPS = [
  { id: M1_FIELDS.id, ...makeFragmentData(M1_FIELDS, CatalogDeckFieldsFragment) },
  { id: M2_FIELDS.id, ...makeFragmentData(M2_FIELDS, CatalogDeckFieldsFragment) },
];

beforeEach(() => {
  vi.clearAllMocks();
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe("<OnboardingStartClient>", () => {
  it("renders a card per cardgroup and the start-with-defaults button", () => {
    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);

    expect(screen.getByTestId("onboarding-deck-m-1")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-deck-m-2")).toBeInTheDocument();
    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-start-defaults")).toBeInTheDocument();
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

  it("shows the coral loading splash on press and keeps it through a successful import", async () => {
    const user = userEvent.setup();
    let resolve: ((o: ImportMasterOutcome) => void) | undefined;
    mockImport.mockReturnValueOnce(
      new Promise<ImportMasterOutcome>((r) => {
        resolve = r;
      }),
    );

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    // No splash before the user picks a cardgroup.
    expect(screen.queryByRole("status")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    // The branded coral splash appears immediately while the import is in flight.
    expect(
      screen.getByRole("status", { name: /setting up your environment/i }),
    ).toBeInTheDocument();

    // It stays through the success navigation (importingId is held until unmount).
    resolve?.({ status: "success", cardgroupId: "cg-9", cardgroupName: "x" });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/cg-9");
    });
    expect(
      screen.getByRole("status", { name: /setting up your environment/i }),
    ).toBeInTheDocument();
  });

  it("removes the coral splash and shows the error banner when the import fails", async () => {
    const user = userEvent.setup();
    let resolve: ((o: ImportMasterOutcome) => void) | undefined;
    mockImport.mockReturnValueOnce(
      new Promise<ImportMasterOutcome>((r) => {
        resolve = r;
      }),
    );

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    // Splash is up while the import is in flight...
    expect(
      screen.getByRole("status", { name: /setting up your environment/i }),
    ).toBeInTheDocument();

    // ...and is torn down when the import fails, leaving the error banner behind.
    resolve?.({ status: "rejected" });
    await waitFor(() => {
      expect(screen.getByTestId("onboarding-import-error")).toBeInTheDocument();
    });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
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

  it("shows the cardgroup-limit copy on a limit_reached outcome", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "limit_reached", limit: 5, current: 5 });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    await waitFor(() => {
      expect(screen.getByTestId("onboarding-import-error")).toHaveTextContent(
        "You already have 5 card groups (maximum 5).",
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

  // --- start-with-defaults path ---

  it("clicking start-with-defaults calls seedDefaultStarters and navigates to /cardgroups on success", async () => {
    const user = userEvent.setup();
    mockSeed.mockResolvedValueOnce({ status: "success", count: 2 });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-start-defaults"));

    expect(mockSeed).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups");
    });
  });

  it("falls back to the create flow when zero default starters were seeded (count === 0)", async () => {
    const user = userEvent.setup();
    mockSeed.mockResolvedValueOnce({ status: "success", count: 0 });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-start-defaults"));

    expect(mockSeed).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
    });
  });

  it("shows the coral splash while seeding and keeps it through the navigation", async () => {
    const user = userEvent.setup();
    let resolve: ((o: SeedDefaultStartersOutcome) => void) | undefined;
    mockSeed.mockReturnValueOnce(
      new Promise<SeedDefaultStartersOutcome>((r) => {
        resolve = r;
      }),
    );

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("onboarding-start-defaults"));
    expect(screen.getByRole("status", { name: /setting up/i })).toBeInTheDocument();

    resolve?.({ status: "success", count: 2 });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups");
    });
    // Splash stays through unmounting navigation.
    expect(screen.getByRole("status", { name: /setting up/i })).toBeInTheDocument();
  });

  it("shows seedError banner and tears down splash on a rejected seed outcome", async () => {
    const user = userEvent.setup();
    let resolve: ((o: SeedDefaultStartersOutcome) => void) | undefined;
    mockSeed.mockReturnValueOnce(
      new Promise<SeedDefaultStartersOutcome>((r) => {
        resolve = r;
      }),
    );

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-start-defaults"));
    expect(screen.getByRole("status", { name: /setting up/i })).toBeInTheDocument();

    resolve?.({ status: "rejected" });
    await waitFor(() => {
      expect(screen.getByTestId("onboarding-import-error")).toHaveTextContent(
        /could not set up the default decks/i,
      );
    });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows auth banner on a seed auth outcome", async () => {
    const user = userEvent.setup();
    mockSeed.mockResolvedValueOnce({ status: "auth", kind: "unauthenticated" });

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-start-defaults"));

    const banner = await screen.findByTestId("onboarding-import-auth-error");
    expect(banner.querySelector("a")).toHaveAttribute("href", "/login");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("re-entry guard: clicking start-with-defaults while a preset import is in flight is ignored", async () => {
    const user = userEvent.setup();
    let resolve: ((o: ImportMasterOutcome) => void) | undefined;
    mockImport.mockReturnValueOnce(
      new Promise<ImportMasterOutcome>((r) => {
        resolve = r;
      }),
    );

    renderWithIntl(<OnboardingStartClient cardgroups={CARDGROUPS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    // Start-with-defaults button is disabled while import is in flight.
    const defaultsBtn = screen.getByTestId("onboarding-start-defaults");
    expect(defaultsBtn).toBeDisabled();

    // Clicking it does not call seedDefaultStarters.
    await user.click(defaultsBtn);
    expect(mockSeed).not.toHaveBeenCalled();

    resolve?.({ status: "success", cardgroupId: "cg-9", cardgroupName: "x" });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/cg-9");
    });
  });
});

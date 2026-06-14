// @vitest-environment jsdom
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ImportMasterOutcome } from "@/app/catalog/use-import-master";
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

const DECKS = [
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
  {
    __typename: "MasterCardgroup" as const,
    id: "m-2",
    name: "JLPT N3 Kanji",
    description: null,
    language: "ja",
    level: null,
    category: null,
    cardCount: 100,
  },
];

beforeEach(() => {
  vi.clearAllMocks();
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe("<OnboardingStartClient>", () => {
  it("renders a card per deck and the create-your-own link", () => {
    renderWithIntl(<OnboardingStartClient decks={DECKS} />);

    expect(screen.getByTestId("onboarding-deck-m-1")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-deck-m-2")).toBeInTheDocument();
    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-create-link")).toHaveAttribute(
      "href",
      "/cardgroups/new?welcome=1",
    );
  });

  it("imports the chosen deck and navigates to /learn/{id} on success", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({
      status: "success",
      cardgroupId: "cg-9",
      cardgroupName: "Business English",
    });

    renderWithIntl(<OnboardingStartClient decks={DECKS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    expect(mockImport).toHaveBeenCalledWith("m-1");
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/cg-9");
    });
  });

  it("shows a generic error banner and does not navigate on a rejected import", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "rejected" });

    renderWithIntl(<OnboardingStartClient decks={DECKS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    await waitFor(() => {
      expect(screen.getByTestId("onboarding-import-error")).toBeInTheDocument();
    });
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the not-found copy on a not_found outcome", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "not_found" });

    renderWithIntl(<OnboardingStartClient decks={DECKS} />);
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

    renderWithIntl(<OnboardingStartClient decks={DECKS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));

    const banner = await screen.findByTestId("onboarding-import-auth-error");
    expect(banner.querySelector("a")).toHaveAttribute("href", "/login");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the auth banner with a sign-in link on a forbidden outcome", async () => {
    const user = userEvent.setup();
    mockImport.mockResolvedValueOnce({ status: "auth", kind: "forbidden" });

    renderWithIntl(<OnboardingStartClient decks={DECKS} />);
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

    renderWithIntl(<OnboardingStartClient decks={DECKS} />);
    await user.click(screen.getByTestId("onboarding-deck-m-1"));
    await user.click(screen.getByTestId("onboarding-deck-m-2"));

    expect(mockImport).toHaveBeenCalledTimes(1);

    resolve?.({ status: "success", cardgroupId: "cg-9", cardgroupName: "x" });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/cg-9");
    });
  });
});

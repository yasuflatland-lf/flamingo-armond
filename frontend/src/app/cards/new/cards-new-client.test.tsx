// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  CreateCardDocument,
  SetLastViewedCardgroupDocument,
  UpdateCardDocument,
} from "@/generated/graphql";
import { installApolloMockLeakSpy } from "../../../../__tests__/utils/mock-apollo-paginated";
import CardsNewClient from "./cards-new-client";

// ---------------------------------------------------------------------------
// Captured picker props — populated by the CardgroupPickerSheet mock below.
// Only used by the CardgroupPickerSheet prop-wiring describe block; the
// navigate-on-success tests do not use this mock (they rely on the real
// component with the picker closed).
// ---------------------------------------------------------------------------
type PickerSheetProps = React.ComponentProps<
  typeof import("@/components/cardgroups/cardgroup-picker-sheet").default
>;
let capturedPickerProps: PickerSheetProps | null = null;

vi.mock("@/components/cardgroups/cardgroup-picker-sheet", () => ({
  default: (
    props: React.ComponentProps<
      typeof import("@/components/cardgroups/cardgroup-picker-sheet").default
    >,
  ) => {
    capturedPickerProps = props;
    return null;
  },
}));

// ---------------------------------------------------------------------------
// next/navigation + next/link stubs
// ---------------------------------------------------------------------------

const mockPush = vi.fn();
const mockReplace = vi.fn();
let mockSearchParamsValue = "";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useSearchParams: () => new URLSearchParams(mockSearchParamsValue),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_ID = "cg-1";
const CG_NAME = "Spanish 101";

const myCardgroups = [
  { id: CG_ID, name: CG_NAME },
  { id: "cg-2", name: "French 101" },
];

function makeCreateMock(args: {
  front: string;
  back: string;
  cardgroupId?: string;
  cardId?: string;
  onCalled?: () => void;
  /** When set, returns the `CardDuplicateFrontError` union variant as data. */
  duplicate?: { existingCardId: string; existingBack: string };
  errors?: GraphQLError[];
  networkError?: Error;
}): MockedResponse {
  const {
    front,
    back,
    cardgroupId = CG_ID,
    cardId = "card-new-1",
    onCalled,
    duplicate,
    errors,
    networkError,
  } = args;
  if (networkError) {
    return {
      request: {
        query: CreateCardDocument,
        variables: { input: { cardgroupId, front, back } },
      },
      error: networkError,
    };
  }
  return {
    request: {
      query: CreateCardDocument,
      variables: { input: { cardgroupId, front, back } },
    },
    result: () => {
      onCalled?.();
      if (errors) {
        return { errors };
      }
      if (duplicate) {
        return {
          data: {
            createCard: {
              __typename: "CardDuplicateFrontError" as const,
              message: "A card with this front already exists in this cardgroup",
              existingCardId: duplicate.existingCardId,
              existingBack: duplicate.existingBack,
            },
          },
        };
      }
      return {
        data: {
          createCard: {
            __typename: "CreateCardSuccess" as const,
            card: {
              __typename: "Card" as const,
              id: cardId,
              front,
              back,
              due: "2026-04-30T00:00:00Z",
              state: 0,
              cardgroupId,
            },
          },
        },
      };
    },
  };
}

function makeUpdateMock(args: {
  id: string;
  back: string;
  cardgroupId?: string;
  front?: string;
  onCalled?: () => void;
  errors?: GraphQLError[];
  networkError?: Error;
}): MockedResponse {
  const { id, back, cardgroupId = CG_ID, front = "apple", onCalled, errors, networkError } = args;
  if (networkError) {
    return {
      request: {
        query: UpdateCardDocument,
        variables: { id, input: { back } },
      },
      error: networkError,
    };
  }
  return {
    request: {
      query: UpdateCardDocument,
      variables: { id, input: { back } },
    },
    result: () => {
      onCalled?.();
      if (errors) {
        return { errors };
      }
      return {
        data: {
          updateCard: {
            __typename: "UpdateCardPayload" as const,
            card: {
              __typename: "Card" as const,
              id,
              front,
              back,
              due: "2026-04-30T00:00:00Z",
              state: 0,
              cardgroupId,
            },
          },
        },
      };
    },
  };
}

function makePersistMock(
  args: {
    cardgroupId?: string;
    onCalled?: () => void;
    errors?: GraphQLError[];
    networkError?: Error;
  } = {},
): MockedResponse {
  const { cardgroupId = CG_ID, onCalled, errors, networkError } = args;
  if (networkError) {
    return {
      request: {
        query: SetLastViewedCardgroupDocument,
        variables: { cardgroupId },
      },
      error: networkError,
    };
  }
  return {
    request: {
      query: SetLastViewedCardgroupDocument,
      variables: { cardgroupId },
    },
    result: () => {
      onCalled?.();
      if (errors) {
        return { errors };
      }
      return {
        data: {
          setLastViewedCardgroup: {
            __typename: "User" as const,
            id: "u-1",
            lastViewedCardgroup: {
              __typename: "Cardgroup" as const,
              id: cardgroupId,
            },
          },
        },
      };
    },
  };
}

function renderClient(
  opts: {
    initialCardgroupId?: string | null;
    forcePickerOpen?: boolean;
    mocks?: MockedResponse[];
  } = {},
) {
  const { initialCardgroupId = CG_ID, forcePickerOpen = false, mocks = [] } = opts;
  return render(
    <MockedProvider mocks={mocks}>
      <CardsNewClient
        initialCardgroupId={initialCardgroupId}
        forcePickerOpen={forcePickerOpen}
        myCardgroups={myCardgroups}
      />
    </MockedProvider>,
  );
}

async function fillAndSubmit(front: string, back: string) {
  const user = userEvent.setup();
  const frontInput = screen.getByLabelText(/front/i);
  const backInput = screen.getByLabelText(/back/i);
  await user.clear(frontInput);
  await user.type(frontInput, front);
  await user.clear(backInput);
  await user.type(backInput, back);
  await user.click(screen.getByRole("button", { name: /add card/i }));
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

let leakSpy: ReturnType<typeof installApolloMockLeakSpy>;
let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  mockPush.mockClear();
  mockReplace.mockClear();
  mockSearchParamsValue = "";
  capturedPickerProps = null;
  // Capture MockedProvider unmatched-mock leak warnings — assertNoLeaks() in
  // afterEach turns them into hard failures.
  leakSpy = installApolloMockLeakSpy({
    operationNames: ["CreateCard", "SetLastViewedCardgroup", "UpdateCard"],
  });
  // Track non-leak warnings so we can introspect [cards-new] persist failures.
  consoleWarnSpy = vi.spyOn(console, "warn");
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  // Always reset to real timers — a test that throws while fake timers are
  // installed would otherwise leak fake timers into subsequent tests and
  // every userEvent / Apollo mutation would silently hang.
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  // Restore outer spy first so console.warn is back to leakSpy's mock, then
  // restore leakSpy so console.warn is back to the real implementation.
  // Reversing the order would leave leakSpy's mock installed permanently.
  consoleWarnSpy.mockRestore();
  leakSpy.teardown();
  consoleErrorSpy.mockRestore();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("<CardsNewClient> — navigate-on-success", () => {
  it("calls router.push with default cardgroup cards path after a successful submit (no ?return=)", async () => {
    renderClient({
      mocks: [makeCreateMock({ front: "Hello", back: "Hola" }), makePersistMock()],
    });

    await fillAndSubmit("Hello", "Hola");

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CG_ID}/cards`);
    });
  });

  it("fires SetLastViewedCardgroup exactly once with the current cardgroup id", async () => {
    const persistCalled = vi.fn();
    renderClient({
      mocks: [
        makeCreateMock({ front: "Hello", back: "Hola" }),
        makePersistMock({ onCalled: persistCalled }),
      ],
    });

    await fillAndSubmit("Hello", "Hola");

    await waitFor(() => {
      expect(persistCalled).toHaveBeenCalledTimes(1);
    });
  });

  it("logs [cards-new] warning and still calls router.push when setLastViewed rejects", async () => {
    // leakSpy (inner spy, silent=true) already swallows console output, so no
    // extra mockImplementation is needed here. Calling mockImplementation on the
    // outer consoleWarnSpy would break the call chain into leakSpy and silence
    // Apollo unmatched-mock leak detection for the rest of this test.
    renderClient({
      mocks: [
        makeCreateMock({ front: "Hello", back: "Hola" }),
        makePersistMock({
          errors: [
            new GraphQLError("forbidden", {
              extensions: { code: "BAD_USER_INPUT" },
            }),
          ],
        }),
      ],
    });

    await fillAndSubmit("Hello", "Hola");

    // Navigation still happens even if setLastViewed rejects (fire-and-forget).
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CG_ID}/cards`);
    });
    // No alert banner from createCard (which succeeded).
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    await waitFor(() => {
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "[cards-new] setLastViewedCardgroup failed",
        expect.objectContaining({ cardgroupId: CG_ID }),
      );
    });
  });

  it("does NOT call router.push when createCard rejects; surfaces error banner", async () => {
    renderClient({
      mocks: [
        makeCreateMock({
          front: "Hello",
          back: "Hola",
          errors: [
            new GraphQLError("backend exploded", {
              extensions: { code: "INTERNAL_SERVER_ERROR" },
            }),
          ],
        }),
      ],
    });

    await fillAndSubmit("Hello", "Hola");

    // Banner from CardForm's getBackendErrorBanner.
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    // Form values are preserved so the user can retry.
    expect((screen.getByLabelText(/front/i) as HTMLInputElement).value).toBe("Hello");
    expect((screen.getByLabelText(/back/i) as HTMLInputElement).value).toBe("Hola");
    // No navigation on rejection.
    expect(mockPush).not.toHaveBeenCalled();

    await waitFor(() => {
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "[cards-new-client] create card rejection",
        expect.objectContaining({
          message: expect.any(String),
          err: expect.anything(),
        }),
      );
    });
  });

  it('renders a "Done" link to /cardgroups/<currentId>/cards when currentId is set', () => {
    renderClient({ initialCardgroupId: CG_ID });

    const done = screen.getByRole("link", { name: /done/i });
    expect(done).toHaveAttribute("href", `/cardgroups/${CG_ID}/cards`);
  });

  it('does NOT render the "Done" link when currentId is null', () => {
    renderClient({
      initialCardgroupId: null,
      // forcePickerOpen would render the picker which queries MyCardgroups; keep
      // it closed here so MockedProvider does not need an extra mock.
      forcePickerOpen: false,
    });

    expect(screen.queryByRole("link", { name: /done/i })).not.toBeInTheDocument();
  });

  it("uses URL cardgroup id (not initialCardgroupId) for setLastViewed and router.push after picker switch", async () => {
    // Simulate the router having already written cg-2 into the URL (e.g. the
    // user picked "French 101" via the picker and the URL reflects that).
    // initialCardgroupId is still cg-1 (server-resolved before the client switch).
    mockSearchParamsValue = "cardgroup=cg-2";

    const persistCalled = vi.fn();
    renderClient({
      initialCardgroupId: CG_ID,
      mocks: [
        makeCreateMock({ cardgroupId: "cg-2", front: "Bonjour", back: "Hello", cardId: "c-fr-1" }),
        makePersistMock({ cardgroupId: "cg-2", onCalled: persistCalled }),
      ],
    });

    // The component should show the French 101 chip because the URL wins.
    expect(screen.getByText("French 101")).toBeInTheDocument();

    await fillAndSubmit("Bonjour", "Hello");

    // SetLastViewedCardgroup must be called with cg-2 (the URL-driven id).
    await waitFor(() => {
      expect(persistCalled).toHaveBeenCalledTimes(1);
    });

    // router.push must navigate to cg-2's cards list, not cg-1's.
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups/cg-2/cards");
    });
  });
});

describe("<CardsNewClient> — duplicate-front overwrite flow", () => {
  it("shows duplicate dialog with side-by-side comparison when create returns CardDuplicateFrontError", async () => {
    renderClient({
      mocks: [
        makeCreateMock({
          front: "apple",
          back: "new back text",
          duplicate: {
            existingCardId: "existing-id",
            existingBack: "existing back text",
          },
        }),
      ],
    });

    await fillAndSubmit("apple", "new back text");

    // Dialog title acts as the discriminator.
    await screen.findByText("Card already exists");
    // Both sides of the comparison must be visible to make the choice informed.
    expect(screen.getByText("existing back text")).toBeInTheDocument();
    expect(screen.getByText("new back text")).toBeInTheDocument();
    // No navigation for the failed create.
    expect(mockPush).not.toHaveBeenCalled();
    // Duplicate-front is routine validation: the create-rejection log path must
    // be skipped so operators are not paged for a normal collision.
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(
      "[cards-new-client] create card rejection",
      expect.anything(),
    );
    // Dialog itself does not surface an inline error banner unless updateCard
    // fails; CardForm's banner is also empty for field-only BAD_USER_INPUT.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("confirm overwrite calls updateCard and navigates to the cardgroup cards list", async () => {
    const updateCalled = vi.fn();
    renderClient({
      mocks: [
        makeCreateMock({
          front: "apple",
          back: "new back text",
          duplicate: {
            existingCardId: "existing-id",
            existingBack: "existing back text",
          },
        }),
        // Second MockedResponse entry consumed by the overwrite click.
        makeUpdateMock({
          id: "existing-id",
          back: "new back text",
          onCalled: updateCalled,
        }),
        // Mirror create-success path: setLastViewed fire-and-forget.
        makePersistMock(),
      ],
    });

    await fillAndSubmit("apple", "new back text");

    const user = userEvent.setup();
    const confirmBtn = await screen.findByRole("button", { name: "Overwrite" });
    await user.click(confirmBtn);

    // updateCard mock was consumed exactly once.
    await waitFor(() => {
      expect(updateCalled).toHaveBeenCalledTimes(1);
    });
    // Dialog closes.
    await waitFor(() => {
      expect(screen.queryByText("Card already exists")).not.toBeInTheDocument();
    });
    // Navigation happens after successful overwrite.
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CG_ID}/cards`);
    });
  });

  it("cancel keeps form intact and does not call updateCard", async () => {
    renderClient({
      mocks: [
        makeCreateMock({
          front: "apple",
          back: "new back text",
          duplicate: {
            existingCardId: "existing-id",
            existingBack: "existing back text",
          },
        }),
        // No updateCard mock — the leak spy in afterEach would catch a stray call.
      ],
    });

    await fillAndSubmit("apple", "new back text");

    const user = userEvent.setup();
    const cancelBtn = await screen.findByRole("button", { name: "Cancel" });
    await user.click(cancelBtn);

    // Dialog closes.
    await waitFor(() => {
      expect(screen.queryByText("Card already exists")).not.toBeInTheDocument();
    });
    // Form values preserved so the user can edit `front` and resubmit.
    expect((screen.getByLabelText(/front/i) as HTMLInputElement).value).toBe("apple");
    expect((screen.getByLabelText(/back/i) as HTMLInputElement).value).toBe("new back text");
    // No navigation on cancel.
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("overwrite mutation error keeps dialog open and surfaces inline error", async () => {
    renderClient({
      mocks: [
        makeCreateMock({
          front: "apple",
          back: "new back text",
          duplicate: {
            existingCardId: "existing-id",
            existingBack: "existing back text",
          },
        }),
        makeUpdateMock({
          id: "existing-id",
          back: "new back text",
          networkError: new Error("network failure"),
        }),
      ],
    });

    await fillAndSubmit("apple", "new back text");

    const user = userEvent.setup();
    const confirmBtn = await screen.findByRole("button", { name: "Overwrite" });
    await user.click(confirmBtn);

    // Dialog stays mounted.
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(screen.getByText("Card already exists")).toBeInTheDocument();
    // Form is NOT reset (overwrite failed, so the user can retry).
    expect((screen.getByLabelText(/front/i) as HTMLInputElement).value).toBe("apple");
    expect((screen.getByLabelText(/back/i) as HTMLInputElement).value).toBe("new back text");
    // No navigation on overwrite failure.
    expect(mockPush).not.toHaveBeenCalled();
    // The inline error log carries the structured shape.
    await waitFor(() => {
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "[cards-new-client] overwrite card rejection",
        expect.objectContaining({
          message: expect.any(String),
          err: expect.anything(),
        }),
      );
    });
  });

  it("shows field-level error from updateCard back validator inline in the dialog", async () => {
    // Two MockedResponse entries: the first triggers the duplicate dialog,
    // the second is consumed by the overwrite click and rejects with a
    // BAD_USER_INPUT error carrying field=back. The dialog must render the
    // backend message verbatim instead of the generic fallback.
    const backValidatorMessage = "back must be at most 4096 characters";
    renderClient({
      mocks: [
        makeCreateMock({
          front: "apple",
          back: "new back text",
          duplicate: {
            existingCardId: "existing-id",
            existingBack: "existing back text",
          },
        }),
        makeUpdateMock({
          id: "existing-id",
          back: "new back text",
          errors: [
            new GraphQLError(backValidatorMessage, {
              extensions: { code: "BAD_USER_INPUT", field: "back" },
            }),
          ],
        }),
      ],
    });

    await fillAndSubmit("apple", "new back text");

    const user = userEvent.setup();
    const confirmBtn = await screen.findByRole("button", { name: "Overwrite" });
    await user.click(confirmBtn);

    // Dialog stays open and the backend's field-level message replaces the
    // generic fallback.
    await waitFor(() => {
      expect(screen.getByText(backValidatorMessage)).toBeInTheDocument();
    });
    expect(screen.getByText("Card already exists")).toBeInTheDocument();
    // The generic fallback must NOT be shown when a field-level message is
    // available — that was the original bug.
    expect(screen.queryByText("Overwrite failed. Please try again.")).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// CardgroupPickerSheet prop wiring
// ---------------------------------------------------------------------------

describe("<CardsNewClient> — CardgroupPickerSheet prop wiring", () => {
  it('passes createReturnTo="/cards/new" to CardgroupPickerSheet', () => {
    renderClient({ initialCardgroupId: CG_ID });

    expect(capturedPickerProps).not.toBeNull();
    expect(capturedPickerProps?.createReturnTo).toBe("/cards/new");
  });

  it("onSelect calls router.replace with the new cardgroup path and scroll:false (no ?return=)", () => {
    // mockSearchParamsValue is "" (reset in beforeEach) — no return param present.
    renderClient({ initialCardgroupId: CG_ID, forcePickerOpen: false });

    expect(capturedPickerProps).not.toBeNull();
    capturedPickerProps?.onSelect("cg-2");

    expect(mockReplace).toHaveBeenCalledWith("/cards/new?cardgroup=cg-2", { scroll: false });
    // Confirm the URL does not include a stray &return= when none was present.
    expect(mockReplace).not.toHaveBeenCalledWith(
      expect.stringContaining("return="),
      expect.anything(),
    );
  });

  it("onSelect preserves the existing ?return= query param when switching cardgroups", () => {
    // Simulate arriving via /cards/new?cardgroup=cg-1&return=/learn/cg-1
    mockSearchParamsValue = "cardgroup=cg-1&return=/learn/cg-1";
    renderClient({ initialCardgroupId: CG_ID, forcePickerOpen: false });

    expect(capturedPickerProps).not.toBeNull();
    capturedPickerProps?.onSelect("cg-2");

    // The rebuilt URL must carry both the new cardgroup id and the sanitized
    // return path. encodeURIComponent("/learn/cg-1") === "%2Flearn%2Fcg-1".
    expect(mockReplace).toHaveBeenCalledWith("/cards/new?cardgroup=cg-2&return=%2Flearn%2Fcg-1", {
      scroll: false,
    });
  });

  it("onSelect does NOT include ?return= when the return param is an external URL (blocked by sanitizeReturnTo)", () => {
    // sanitizeReturnTo rejects external URLs; the rebuilt URL must not carry
    // a poisoned return= value.
    mockSearchParamsValue = "cardgroup=cg-1&return=https://evil.example.com";
    renderClient({ initialCardgroupId: CG_ID, forcePickerOpen: false });

    expect(capturedPickerProps).not.toBeNull();
    capturedPickerProps?.onSelect("cg-2");

    expect(mockReplace).toHaveBeenCalledWith("/cards/new?cardgroup=cg-2", { scroll: false });
    expect(mockReplace).not.toHaveBeenCalledWith(
      expect.stringContaining("evil.example.com"),
      expect.anything(),
    );
  });

  it("passes open=true to CardgroupPickerSheet when forcePickerOpen is true", () => {
    renderClient({ initialCardgroupId: null, forcePickerOpen: true });

    expect(capturedPickerProps?.open).toBe(true);
  });

  it("passes open=false to CardgroupPickerSheet when forcePickerOpen is false", () => {
    renderClient({ initialCardgroupId: CG_ID, forcePickerOpen: false });

    expect(capturedPickerProps?.open).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// ?return= query parameter — open-redirect guard and post-create navigation
// ---------------------------------------------------------------------------

describe("<CardsNewClient> — ?return= navigation", () => {
  it("pushes to the sanitized ?return= path after a successful create", async () => {
    // A valid internal path is allowed through; router.push should use it.
    mockSearchParamsValue = `cardgroup=${CG_ID}&return=/learn/abc`;

    renderClient({
      mocks: [makeCreateMock({ front: "Hello", back: "Hola" }), makePersistMock()],
    });

    await fillAndSubmit("Hello", "Hola");

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/learn/abc");
    });
  });

  it("falls back to /cardgroups/<id>/cards when no ?return= param is present", async () => {
    // No return param — mockSearchParamsValue is reset to "" in beforeEach.
    renderClient({
      mocks: [makeCreateMock({ front: "Hello", back: "Hola" }), makePersistMock()],
    });

    await fillAndSubmit("Hello", "Hola");

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CG_ID}/cards`);
    });
  });

  it("falls back to /cardgroups/<id>/cards when ?return= is an external URL (open-redirect blocked)", async () => {
    // sanitizeReturnTo returns null for external URLs; the fallback path is used.
    mockSearchParamsValue = `cardgroup=${CG_ID}&return=https://evil.example.com`;

    renderClient({
      mocks: [makeCreateMock({ front: "Hello", back: "Hola" }), makePersistMock()],
    });

    await fillAndSubmit("Hello", "Hola");

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(`/cardgroups/${CG_ID}/cards`);
    });
    // The evil URL must never be passed to router.push.
    expect(mockPush).not.toHaveBeenCalledWith("https://evil.example.com");
  });
});

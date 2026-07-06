// @vitest-environment happy-dom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";
import { UpdateNewCardRatioDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { NewCardRatioSection } from "./new-card-ratio-section";

// The section is unit-tested against a lightweight Slider double so the
// label-update path (onValueChange) can be exercised independently of the save
// path (onValueCommit). Radix fires BOTH callbacks on every keyboard/pointer
// interaction, which would make "onValueChange fires no mutation" unobservable
// through the real primitive. The real wrapper's aria/keyboard wiring is
// covered separately in src/components/ui/slider.test.tsx.
vi.mock("@/components/ui/slider", () => ({
  Slider: ({
    value,
    onValueChange,
    onValueCommit,
    "aria-valuetext": ariaValueText,
  }: {
    value: number[];
    onValueChange?: (value: number[]) => void;
    onValueCommit?: (value: number[]) => void;
    "aria-valuetext"?: string;
  }) => (
    <div>
      <div data-testid="slider-value">{value[0]}</div>
      <div data-testid="slider-valuetext">{ariaValueText}</div>
      <button type="button" data-testid="change-90" onClick={() => onValueChange?.([90])}>
        change to 90
      </button>
      <button type="button" data-testid="commit-85" onClick={() => onValueCommit?.([85])}>
        commit 85
      </button>
      <button type="button" data-testid="commit-90" onClick={() => onValueCommit?.([90])}>
        commit 90
      </button>
    </div>
  ),
}));

function makeUpdateRatioMock(
  numerator: number,
  opts: { error?: boolean; onCalled?: () => void } = {},
) {
  return {
    request: {
      query: UpdateNewCardRatioDocument,
      variables: { numerator, denominator: 100 },
    },
    result: () => {
      opts.onCalled?.();
      if (opts.error) {
        return {
          errors: [new GraphQLError("not signed in", { extensions: { code: "UNAUTHENTICATED" } })],
        };
      }
      return {
        data: {
          updateNewCardRatio: {
            __typename: "User" as const,
            id: "user-1",
            newCardRatio: {
              __typename: "NewCardRatio" as const,
              numerator,
              denominator: 100,
            },
          },
        },
      };
    },
  };
}

describe("<NewCardRatioSection>", () => {
  it("derives the initial percent from initialRatio ({4,5} -> 80)", () => {
    renderWithIntl(
      <MockedProvider mocks={[]}>
        <NewCardRatioSection initialRatio={{ numerator: 4, denominator: 5 }} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("slider-value")).toHaveTextContent("80");
    expect(screen.getByText("New 80%")).toBeInTheDocument();
    expect(screen.getByText("Review 20%")).toBeInTheDocument();
    expect(screen.getByTestId("slider-valuetext")).toHaveTextContent("New 80%, review 20%");
  });

  it("derives the initial percent from a reduced fraction ({3,20} -> 15)", () => {
    renderWithIntl(
      <MockedProvider mocks={[]}>
        <NewCardRatioSection initialRatio={{ numerator: 3, denominator: 20 }} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("slider-value")).toHaveTextContent("15");
    expect(screen.getByText("New 15%")).toBeInTheDocument();
    expect(screen.getByText("Review 85%")).toBeInTheDocument();
  });

  it("onValueChange updates the label but fires no mutation", async () => {
    const user = userEvent.setup();
    const onCalled = vi.fn();

    renderWithIntl(
      <MockedProvider mocks={[makeUpdateRatioMock(90, { onCalled })]}>
        <NewCardRatioSection initialRatio={{ numerator: 4, denominator: 5 }} />
      </MockedProvider>,
    );

    await user.click(screen.getByTestId("change-90"));

    // The label reflects the dragged value immediately.
    expect(screen.getByText("New 90%")).toBeInTheDocument();
    expect(screen.getByText("Review 10%")).toBeInTheDocument();
    expect(screen.getByTestId("slider-valuetext")).toHaveTextContent("New 90%, review 10%");

    // No save happens until release, so the mutation must not have fired.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onCalled).not.toHaveBeenCalled();
  });

  it("onValueCommit fires updateNewCardRatio with { numerator: percent, denominator: 100 }", async () => {
    const user = userEvent.setup();
    const onCalled = vi.fn();

    // The mock only matches when the exact { numerator: 85, denominator: 100 }
    // variables are sent, so a matched call proves the wire format.
    renderWithIntl(
      <MockedProvider mocks={[makeUpdateRatioMock(85, { onCalled })]}>
        <NewCardRatioSection initialRatio={{ numerator: 4, denominator: 5 }} />
      </MockedProvider>,
    );

    await user.click(screen.getByTestId("commit-85"));

    await waitFor(() => expect(onCalled).toHaveBeenCalledTimes(1));
  });

  it("rolls the label back to the last committed value and shows the banner when a commit fails", async () => {
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const user = userEvent.setup();
    const okCalled = vi.fn();
    // Two MockedResponse entries: a successful commit that advances the
    // committed value to 85, then a failing commit at 90 that must roll the
    // slider back to 85 (not the original 80) and surface the banner.
    const mocks = [
      makeUpdateRatioMock(85, { onCalled: okCalled }),
      makeUpdateRatioMock(90, { error: true }),
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <NewCardRatioSection initialRatio={{ numerator: 4, denominator: 5 }} />
      </MockedProvider>,
    );

    await user.click(screen.getByTestId("commit-85"));
    await waitFor(() => expect(okCalled).toHaveBeenCalledTimes(1));
    // Let the in-flight `loading` flag settle back to false before the next commit.
    await new Promise((resolve) => setTimeout(resolve, 0));
    await waitFor(() => expect(screen.getByTestId("slider-value")).toHaveTextContent("85"));

    await user.click(screen.getByTestId("commit-90"));

    const alert = await screen.findByRole("alert");
    expect(alert).toBe(screen.getByTestId("new-card-ratio-error"));
    expect(alert).toHaveTextContent(/.+/);

    // Rolled back to the last server-confirmed value, not the original 80.
    await waitFor(() => expect(screen.getByTestId("slider-value")).toHaveTextContent("85"));

    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[profile] updateNewCardRatio rejected",
      expect.objectContaining({ codes: expect.arrayContaining(["UNAUTHENTICATED"]) }),
    );
    consoleWarnSpy.mockRestore();
  });

  it("does not carry optimisticResponse in the updateNewCardRatio mutation", () => {
    // Static assertion: updateNewCardRatio can return UNAUTHENTICATED (a typed
    // GraphQL error). Apollo v3 does not reliably roll back optimistic writes on
    // typed GraphQL errors — only on network errors. So no `optimisticResponse`
    // must appear in the mutate call; the component rolls back manually in .catch.
    // See .claude/rules/pagination.md § "Drop `optimisticResponse` for mutations
    // that can fail with typed GraphQL errors".
    const source = NewCardRatioSection.toString();

    const mutateStart = source.indexOf("updateRatio({");
    expect(mutateStart).toBeGreaterThan(-1);

    const catchIdx = source.indexOf("} catch (", mutateStart);
    expect(catchIdx).toBeGreaterThan(-1);

    const mutateBlock = source.slice(mutateStart, catchIdx);
    expect(mutateBlock).not.toContain("optimisticResponse");
  });
});

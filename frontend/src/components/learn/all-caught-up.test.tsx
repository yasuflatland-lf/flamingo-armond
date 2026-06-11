// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { AllCaughtUp } from "./all-caught-up";

describe("<AllCaughtUp>", () => {
  it("renders the caught-up message and cardgroups link", () => {
    renderWithIntl(<AllCaughtUp />);

    expect(
      screen.getByRole("heading", { name: "Today's learning is complete" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Come back when the next review is due/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to cardgroups" })).toHaveAttribute(
      "href",
      "/cardgroups",
    );
  });

  it("does not render a Study again button by default", () => {
    renderWithIntl(<AllCaughtUp />);
    expect(screen.queryByRole("button", { name: "Study again" })).not.toBeInTheDocument();
  });

  it("renders custom heading and message when provided", () => {
    renderWithIntl(<AllCaughtUp heading="Practice complete" message="Another round awaits." />);

    expect(screen.getByRole("heading", { name: "Practice complete" })).toBeInTheDocument();
    expect(screen.getByText("Another round awaits.")).toBeInTheDocument();
    // The back link is always present regardless of custom copy.
    expect(screen.getByRole("link", { name: "Back to cardgroups" })).toHaveAttribute(
      "href",
      "/cardgroups",
    );
  });

  it("renders a Study again button only when onStudyAgain is provided", () => {
    renderWithIntl(<AllCaughtUp onStudyAgain={() => {}} />);
    expect(screen.getByRole("button", { name: "Study again" })).toBeInTheDocument();
  });

  it("fires onStudyAgain when the Study again button is clicked", async () => {
    const user = userEvent.setup();
    const onStudyAgain = vi.fn();
    renderWithIntl(<AllCaughtUp onStudyAgain={onStudyAgain} />);

    await user.click(screen.getByRole("button", { name: "Study again" }));

    expect(onStudyAgain).toHaveBeenCalledOnce();
  });
});

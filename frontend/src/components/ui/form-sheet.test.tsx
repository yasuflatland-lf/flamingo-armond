// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useIsMobile } from "@/hooks/use-mobile";
import { FormSheet, useFormSheetClose } from "./form-sheet";

vi.mock("@/hooks/use-mobile", () => ({
  useIsMobile: vi.fn(),
}));

function ContextCancelButton() {
  const close = useFormSheetClose();
  return (
    <button type="button" onClick={close}>
      Cancel
    </button>
  );
}

function getOpenOverlay() {
  const overlay = Array.from(document.querySelectorAll<HTMLElement>("[data-state='open']")).find(
    (element) =>
      element.hasAttribute("data-vaul-overlay") ||
      (typeof element.className === "string" && element.className.includes("bg-black/80")),
  );

  expect(overlay).toBeDefined();
  return overlay as HTMLElement;
}

describe("<FormSheet>", () => {
  beforeEach(() => {
    vi.mocked(useIsMobile).mockReturnValue(false);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders a desktop right sheet when useIsMobile returns false", () => {
    render(
      <FormSheet open onOpenChange={vi.fn()} title="Edit card">
        <p>Form body</p>
      </FormSheet>,
    );

    const sheet = screen.getByRole("dialog");
    expect(sheet).toBeInTheDocument();
    expect(sheet).toHaveClass("data-[state=open]:slide-in-from-right");
    expect(screen.getByRole("heading", { name: "Edit card" })).toBeInTheDocument();
    expect(screen.getByText("Form body")).toBeInTheDocument();
  });

  it.each([
    ["sm", "sm:max-w-md"],
    ["md", "sm:max-w-lg"],
    ["lg", "sm:max-w-2xl"],
  ] as const)("applies the size=%s width to the desktop sheet", (size, expectedClass) => {
    render(
      <FormSheet open onOpenChange={vi.fn()} title="Edit card" size={size}>
        <p>Form body</p>
      </FormSheet>,
    );

    expect(screen.getByRole("dialog")).toHaveClass(expectedClass);
  });

  it("defaults to the medium width when size is not provided", () => {
    render(
      <FormSheet open onOpenChange={vi.fn()} title="Edit card">
        <p>Form body</p>
      </FormSheet>,
    );

    expect(screen.getByRole("dialog")).toHaveClass("sm:max-w-lg");
  });

  it("constrains the desktop sheet body so tall forms can scroll", () => {
    render(
      <FormSheet open onOpenChange={vi.fn()} title="Edit card">
        <div style={{ height: 2000 }}>Tall form</div>
      </FormSheet>,
    );

    expect(screen.getByTestId("form-sheet-body")).toHaveClass("flex-1", "overflow-y-auto");
  });

  it("renders a mobile drawer dialog when useIsMobile returns true", () => {
    vi.mocked(useIsMobile).mockReturnValue(true);

    render(
      <FormSheet open onOpenChange={vi.fn()} title="Add card">
        <p>Mobile body</p>
      </FormSheet>,
    );

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Add card" })).toBeInTheDocument();
    expect(screen.getByText("Mobile body")).toBeInTheDocument();
  });

  it("honors a clean guarded close request immediately", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Edit card">
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("routes the desktop dialog close button through guarded close", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Edit card">
        <p>Form body</p>
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Close" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("routes desktop escape dismiss through guarded close", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Edit card">
        <p>Form body</p>
      </FormSheet>,
    );

    await user.keyboard("{Escape}");

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("routes desktop overlay dismiss through guarded close", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Edit card">
        <p>Form body</p>
      </FormSheet>,
    );

    await user.click(getOpenOverlay());

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("routes mobile drawer overlay dismiss through guarded close", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    vi.mocked(useIsMobile).mockReturnValue(true);

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card">
        <p>Mobile body</p>
      </FormSheet>,
    );

    await user.click(getOpenOverlay());

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("routes mobile drawer drag dismiss through guarded close", async () => {
    const onOpenChange = vi.fn();
    vi.mocked(useIsMobile).mockReturnValue(true);

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card">
        <p>Mobile body</p>
      </FormSheet>,
    );

    const drawer = screen.getByRole("dialog");
    drawer.setPointerCapture = vi.fn();
    vi.spyOn(drawer, "getBoundingClientRect").mockReturnValue({
      bottom: 400,
      height: 400,
      left: 0,
      right: 320,
      top: 0,
      width: 320,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerDown(drawer, {
      clientY: 0,
      pageY: 0,
      pointerId: 1,
      pointerType: "touch",
    });
    await waitFor(() => {
      fireEvent.pointerMove(drawer, {
        clientY: 320,
        pageY: 320,
        pointerId: 1,
        pointerType: "touch",
      });
      expect(drawer).toHaveClass("vaul-dragging");
    });
    drawer.style.transform = "matrix(1, 0, 0, 1, 0, 320)";
    fireEvent.pointerUp(drawer, {
      clientY: 320,
      pageY: 320,
      pointerId: 1,
      pointerType: "touch",
    });

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("closes immediately when confirmOnDismiss is true but the form is clean", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Edit card" confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("opens discard confirmation instead of closing when dirty confirmOnDismiss is true", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card" dirty confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText("Discard your changes?")).toBeInTheDocument();
  });

  it("keeps editing from the discard confirmation", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card" dirty confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(screen.getByRole("button", { name: "Keep editing" }));

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(screen.getByRole("heading", { name: "Add card" })).toBeInTheDocument();
  });

  it("discards from the discard confirmation", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card" dirty confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(screen.getByRole("button", { name: "Discard" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("blocks discard confirmation action while submitting", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    const { rerender } = render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card" dirty confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();

    rerender(
      <FormSheet
        open
        onOpenChange={onOpenChange}
        title="Add card"
        dirty
        confirmOnDismiss
        submitting
      >
        <ContextCancelButton />
      </FormSheet>,
    );
    await user.click(screen.getByRole("button", { name: "Discard" }));

    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  });

  it("clears a pending discard confirmation when the parent closes the sheet", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    const { rerender } = render(
      <FormSheet open onOpenChange={onOpenChange} title="Add card" dirty confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();

    rerender(
      <FormSheet open={false} onOpenChange={onOpenChange} title="Add card" dirty confirmOnDismiss>
        <ContextCancelButton />
      </FormSheet>,
    );

    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  });

  it("blocks guarded close requests while submitting", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FormSheet open onOpenChange={onOpenChange} title="Edit card" submitting>
        <ContextCancelButton />
      </FormSheet>,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("constrains the mobile drawer body so tall forms can scroll", () => {
    vi.mocked(useIsMobile).mockReturnValue(true);

    render(
      <FormSheet open onOpenChange={vi.fn()} title="Add card">
        <div style={{ height: 2000 }}>Tall form</div>
      </FormSheet>,
    );

    expect(screen.getByTestId("form-sheet-body")).toHaveClass(
      "max-h-[calc(100dvh-7rem)]",
      "overflow-y-auto",
    );
  });

  it("renders a string-title desktop sheet with accessible description '<title> form'", () => {
    render(
      <FormSheet open onOpenChange={vi.fn()} title="Edit card">
        <p>Body</p>
      </FormSheet>,
    );
    // The SheetDescription carries "<title> form" so screen readers announce the form purpose.
    expect(screen.getByRole("dialog")).toHaveAccessibleDescription("Edit card form");
  });

  describe.each([{ mobile: false }, { mobile: true }])("ReactNode title — mobile=$mobile", ({
    mobile,
  }) => {
    beforeEach(() => {
      vi.mocked(useIsMobile).mockReturnValue(mobile);
    });

    it("exposes both verb and destination in the heading textContent", () => {
      const cardgroupName = "Yasu Cardgroup";
      render(
        <FormSheet
          open
          onOpenChange={vi.fn()}
          title={
            <span
              className="block overflow-hidden text-ellipsis whitespace-nowrap"
              title={cardgroupName}
            >
              <span className="sr-only">Batch import into </span>
              {cardgroupName}
            </span>
          }
        >
          <p>Body</p>
        </FormSheet>,
      );

      // textContent includes sr-only spans, so both the hidden prefix and the
      // visible cardgroup name appear (this is NOT the ARIA-computed name).
      const heading = screen.getByRole("heading");
      expect(heading.textContent).toContain("Batch import into");
      expect(heading.textContent).toContain(cardgroupName);
      // The dialog is labelled by the title element via aria-labelledby; the
      // computed accessible name therefore includes both parts.
      expect(screen.getByRole("dialog")).toHaveAccessibleName(/batch import into.*yasu cardgroup/i);
    });

    it("title span carries truncation classes and tooltip attribute", () => {
      const cardgroupName = "A Very Long Cardgroup Name That Should Truncate";
      render(
        <FormSheet
          open
          onOpenChange={vi.fn()}
          title={
            <span
              className="block overflow-hidden text-ellipsis whitespace-nowrap"
              title={cardgroupName}
            >
              <span className="sr-only">Batch import into </span>
              {cardgroupName}
            </span>
          }
        >
          <p>Body</p>
        </FormSheet>,
      );

      const heading = screen.getByRole("heading");
      const titleSpan = heading.querySelector("span[title]");
      expect(titleSpan).not.toBeNull();
      expect(titleSpan).toHaveAttribute("title", cardgroupName);
      expect(titleSpan).toHaveClass("overflow-hidden", "text-ellipsis", "whitespace-nowrap");
    });

    it("a11yDescription falls back to 'Form' when title is a ReactNode and no description is provided", () => {
      render(
        <FormSheet open onOpenChange={vi.fn()} title={<span>Some ReactNode title</span>}>
          <p>Body</p>
        </FormSheet>,
      );

      // The sr-only description element carries the fallback text "Form".
      expect(screen.getByRole("dialog")).toHaveAccessibleDescription("Form");
    });
  });
});

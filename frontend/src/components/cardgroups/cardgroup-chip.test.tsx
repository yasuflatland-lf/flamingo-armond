// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CardgroupChip } from "./cardgroup-chip";

describe("<CardgroupChip>", () => {
  it("renders the cardgroup name when provided", () => {
    render(<CardgroupChip name="Spanish Vocab" onChangeRequested={vi.fn()} />);
    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
  });

  it('renders "Select cardgroup" placeholder when name is null', () => {
    render(<CardgroupChip name={null} onChangeRequested={vi.fn()} />);
    expect(screen.getByText("Select cardgroup")).toBeInTheDocument();
  });

  it("renders an empty muted span when name is an empty string", () => {
    // `name ?? "Select cardgroup"` uses nullish coalescing: "" is not null/undefined,
    // so the span renders empty. The muted style is applied via isMuted = !name.
    const { container } = render(<CardgroupChip name="" onChangeRequested={vi.fn()} />);
    const span = container.querySelector("span.truncate");
    expect(span).not.toBeNull();
    expect(span?.textContent).toBe("");
    expect(span?.className).toMatch(/text-muted-foreground/);
  });

  it("fires onChangeRequested when the button is clicked", async () => {
    const user = userEvent.setup();
    const onChangeRequested = vi.fn();
    render(<CardgroupChip name="My Group" onChangeRequested={onChangeRequested} />);

    await user.click(screen.getByRole("button"));

    expect(onChangeRequested).toHaveBeenCalledOnce();
  });

  it("renders the ChevronDown icon inside the button", () => {
    render(<CardgroupChip name="My Group" onChangeRequested={vi.fn()} />);
    // lucide-react renders an <svg> inside the button; assert its presence
    const button = screen.getByRole("button");
    expect(button.querySelector("svg")).not.toBeNull();
  });

  it("has a descriptive aria-label that includes the cardgroup name when provided", () => {
    render(<CardgroupChip name="Japanese Kanji" onChangeRequested={vi.fn()} />);
    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-label", 'Change cardgroup (currently "Japanese Kanji")');
  });

  it('has aria-label "Select cardgroup" when name is null', () => {
    render(<CardgroupChip name={null} onChangeRequested={vi.fn()} />);
    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-label", "Select cardgroup");
  });

  it("name span carries the truncate class for overflow handling", () => {
    render(<CardgroupChip name="Very Long Cardgroup Name" onChangeRequested={vi.fn()} />);
    const span = screen.getByText("Very Long Cardgroup Name");
    expect(span.className).toMatch(/truncate/);
  });
});

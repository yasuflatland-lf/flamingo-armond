// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { Slider } from "./slider";

// A controlled wrapper mirroring how NewCardRatioSection drives the slider:
// value is state, and aria-valuetext is recomputed from it on every change.
function ControlledSlider() {
  const [value, setValue] = useState(70);
  return (
    <Slider
      min={5}
      max={80}
      step={5}
      value={[value]}
      aria-label="New card ratio"
      aria-valuetext={`New ${value}%, review ${100 - value}%`}
      onValueChange={([next = value]) => setValue(next)}
    />
  );
}

describe("<Slider>", () => {
  it("forwards aria-label and aria-valuetext onto the role=slider thumb", () => {
    render(<ControlledSlider />);

    const slider = screen.getByRole("slider");
    expect(slider).toHaveAttribute("aria-label", "New card ratio");
    expect(slider).toHaveAttribute("aria-valuetext", "New 70%, review 30%");
    expect(slider).toHaveAttribute("aria-valuenow", "70");
  });

  it("forwards aria-describedby onto the role=slider thumb, not the root", () => {
    render(
      <>
        <Slider
          min={5}
          max={80}
          step={5}
          value={[35]}
          aria-label="New card ratio"
          aria-describedby="ratio-note"
        />
        <p id="ratio-note">A custom ratio is set via the API.</p>
      </>,
    );

    // Radix spreads leftover props onto the root, which carries no ARIA role,
    // so the description would never be announced from there.
    expect(screen.getByRole("slider")).toHaveAttribute("aria-describedby", "ratio-note");
  });

  it("moves the value by step on ArrowRight / ArrowLeft", async () => {
    const user = userEvent.setup();
    render(<ControlledSlider />);

    const slider = screen.getByRole("slider");
    slider.focus();

    await user.keyboard("{ArrowRight}");
    expect(slider).toHaveAttribute("aria-valuenow", "75");
    expect(slider).toHaveAttribute("aria-valuetext", "New 75%, review 25%");

    await user.keyboard("{ArrowLeft}");
    expect(slider).toHaveAttribute("aria-valuenow", "70");
    expect(slider).toHaveAttribute("aria-valuetext", "New 70%, review 30%");
  });
});

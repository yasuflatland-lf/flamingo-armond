// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { Slider } from "./slider";

// A controlled wrapper mirroring how NewCardRatioSection drives the slider:
// value is state, and aria-valuetext is recomputed from it on every change.
function ControlledSlider() {
  const [value, setValue] = useState(80);
  return (
    <Slider
      min={5}
      max={95}
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
    expect(slider).toHaveAttribute("aria-valuetext", "New 80%, review 20%");
    expect(slider).toHaveAttribute("aria-valuenow", "80");
  });

  it("moves the value by step on ArrowRight / ArrowLeft", async () => {
    const user = userEvent.setup();
    render(<ControlledSlider />);

    const slider = screen.getByRole("slider");
    slider.focus();

    await user.keyboard("{ArrowRight}");
    expect(slider).toHaveAttribute("aria-valuenow", "85");
    expect(slider).toHaveAttribute("aria-valuetext", "New 85%, review 15%");

    await user.keyboard("{ArrowLeft}");
    expect(slider).toHaveAttribute("aria-valuenow", "80");
    expect(slider).toHaveAttribute("aria-valuetext", "New 80%, review 20%");
  });
});

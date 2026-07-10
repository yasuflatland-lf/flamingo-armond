// @vitest-environment happy-dom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProgressMeter } from "./progress-meter";

describe("ProgressMeter", () => {
  it("renders a progressbar whose accessible name equals the label", () => {
    render(<ProgressMeter value={10} max={20} label="Acquisition progress" />);
    const bar = screen.getByRole("progressbar", { name: "Acquisition progress" });
    expect(bar).toBeTruthy();
  });

  it("computes aria-valuenow and fill width from value / max", () => {
    render(<ProgressMeter value={120} max={500} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("24");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("24%");
  });

  it("reaches 100 when value equals max", () => {
    render(<ProgressMeter value={210} max={210} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("100");
  });

  it("returns 0 and emits no NaN when max is 0", () => {
    const { container } = render(<ProgressMeter value={5} max={0} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("0");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("0%");
    expect(container.textContent ?? "").not.toContain("NaN");
  });

  it("clamps to 100 when value exceeds max", () => {
    render(<ProgressMeter value={999} max={100} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("100");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("100%");
  });
});

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

  it("clamps a negative value to 0", () => {
    render(<ProgressMeter value={-5} max={100} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("0");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("0%");
  });

  it("renders 0 (no NaN, no false full bar) when value is not finite", () => {
    const { container } = render(<ProgressMeter value={Number.NaN} max={100} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("0");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("0%");
    expect(container.textContent ?? "").not.toContain("NaN");
  });

  it("forwards className onto the track element", () => {
    render(<ProgressMeter value={1} max={2} label="progress" className="h-3" />);
    expect(screen.getByRole("progressbar").className).toContain("h-3");
  });

  it("rounds aria-valuenow but keeps the fill width at full precision", () => {
    render(<ProgressMeter value={1} max={3} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("33");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width.startsWith("33.3")).toBe(true);
  });

  it("exposes a static [0, 100] range", () => {
    render(<ProgressMeter value={1} max={2} label="progress" />);
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuemin")).toBe("0");
    expect(bar.getAttribute("aria-valuemax")).toBe("100");
  });
});

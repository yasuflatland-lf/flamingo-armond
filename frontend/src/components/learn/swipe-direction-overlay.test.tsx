// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SwipeDirectionOverlay } from "./swipe-direction-overlay";

describe("SwipeDirectionOverlay — direction to label mapping", () => {
  it("renders 'Again' for direction left", () => {
    render(<SwipeDirectionOverlay direction="left" intensity={1} />);
    expect(screen.getByText("Again")).toBeInTheDocument();
  });

  it("renders 'Hard' for direction down", () => {
    render(<SwipeDirectionOverlay direction="down" intensity={1} />);
    expect(screen.getByText("Hard")).toBeInTheDocument();
  });

  it("renders 'Easy' for direction right", () => {
    render(<SwipeDirectionOverlay direction="right" intensity={1} />);
    expect(screen.getByText("Easy")).toBeInTheDocument();
  });
});

describe("SwipeDirectionOverlay — color class mapping", () => {
  it("applies red classes for direction left", () => {
    render(<SwipeDirectionOverlay direction="left" intensity={1} />);
    const label = screen.getByText("Again");
    expect(label.className).toContain("border-red-500");
    expect(label.className).toContain("bg-red-500/10");
    expect(label.className).toContain("text-red-700");
  });

  it("applies sky classes for direction down", () => {
    render(<SwipeDirectionOverlay direction="down" intensity={1} />);
    const label = screen.getByText("Hard");
    expect(label.className).toContain("border-sky-500");
    expect(label.className).toContain("bg-sky-500/10");
    expect(label.className).toContain("text-sky-700");
  });

  it("applies emerald classes for direction right", () => {
    render(<SwipeDirectionOverlay direction="right" intensity={1} />);
    const label = screen.getByText("Easy");
    expect(label.className).toContain("border-emerald-500");
    expect(label.className).toContain("bg-emerald-500/10");
    expect(label.className).toContain("text-emerald-700");
  });
});

describe("SwipeDirectionOverlay — intensity clamping and opacity formula", () => {
  function getInnerDiv(container: HTMLElement): HTMLElement {
    // container (render wrapper) > outer overlay div > inner label div
    return container.querySelector("div > div > div") as HTMLElement;
  }

  it("sets opacity to 0.25 when intensity is 0", () => {
    const { container } = render(<SwipeDirectionOverlay direction="right" intensity={0} />);
    expect(getInnerDiv(container).style.opacity).toBe("0.25");
  });

  it("sets opacity to 1 when intensity is 1", () => {
    const { container } = render(<SwipeDirectionOverlay direction="right" intensity={1} />);
    expect(getInnerDiv(container).style.opacity).toBe("1");
  });

  it("sets opacity to 0.625 when intensity is 0.5", () => {
    const { container } = render(<SwipeDirectionOverlay direction="right" intensity={0.5} />);
    expect(getInnerDiv(container).style.opacity).toBe("0.625");
  });

  it("clamps intensity above 1 to 1 (opacity = 1)", () => {
    const { container } = render(<SwipeDirectionOverlay direction="right" intensity={2} />);
    expect(getInnerDiv(container).style.opacity).toBe("1");
  });

  it("clamps intensity below 0 to 0 (opacity = 0.25)", () => {
    const { container } = render(<SwipeDirectionOverlay direction="right" intensity={-1} />);
    expect(getInnerDiv(container).style.opacity).toBe("0.25");
  });
});

describe("SwipeDirectionOverlay — null direction", () => {
  it("returns null (empty DOM) when direction is null", () => {
    const { container } = render(<SwipeDirectionOverlay direction={null} intensity={1} />);
    expect(container).toBeEmptyDOMElement();
  });
});

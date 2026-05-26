// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { BootSplash } from "./boot-splash";

describe("<BootSplash>", () => {
  it("renders the coral background container", () => {
    const { container } = render(<BootSplash />);
    // The outermost div carries the coral brand background class.
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper).not.toBeNull();
    expect(wrapper.className).toContain("bg-[#FF6F79]");
  });

  it("renders an aria-hidden FlamingoMark svg", () => {
    render(<BootSplash />);
    // FlamingoMark renders an <svg> with aria-label="Flamingo" and aria-hidden
    // passed from the BootSplash usage. The aria-hidden attribute hides it from
    // the accessibility tree so screen readers skip the decorative logo.
    const svg = document.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg?.getAttribute("aria-hidden")).toBe("true");
  });
});

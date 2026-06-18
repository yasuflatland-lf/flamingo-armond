// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MasterCardsSection } from "./master-cards-section";

describe("MasterCardsSection (placeholder)", () => {
  it("mounts a stable container keyed to the master id", () => {
    render(<MasterCardsSection masterId="m-1" />);
    const section = screen.getByTestId("master-cards-section");
    expect(section).toBeInTheDocument();
    expect(section).toHaveAttribute("data-master-id", "m-1");
  });
});

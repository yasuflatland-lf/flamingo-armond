// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("root <Loading>", () => {
  it("renders the branded loading splash with an accessible status region", () => {
    render(<Loading />);
    expect(screen.getByRole("status", { name: /loading/i })).toBeInTheDocument();
  });
});

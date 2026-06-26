// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/this-route-does-not-exist",
}));

import NotFound from "./not-found";

afterEach(() => {
  vi.clearAllMocks();
});

describe("<NotFound>", () => {
  it("renders the 404 heading, message, and home link", () => {
    renderWithIntl(<NotFound />);

    expect(screen.getByText("404")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /page not found/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to home/i })).toHaveAttribute("href", "/");
  });
});

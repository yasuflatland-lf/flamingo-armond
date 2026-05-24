// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  // Arbitrary unknown route: the FAB would otherwise resolve to the default
  // "Add new card" action and render.
  usePathname: () => "/this-route-does-not-exist",
}));

import { FabSuppressionProvider } from "@/components/nav/fab-suppression";
import { GlobalFAB } from "@/components/nav/global-fab";
import NotFound from "./not-found";

afterEach(() => {
  vi.clearAllMocks();
});

describe("<NotFound>", () => {
  it("renders the 404 heading, message, and home link", () => {
    render(<NotFound />);

    expect(screen.getByText("404")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /page not found/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to home/i })).toHaveAttribute("href", "/");
  });

  it("suppresses the global FAB while mounted", () => {
    render(
      <FabSuppressionProvider>
        <NotFound />
        <GlobalFAB />
      </FabSuppressionProvider>,
    );

    expect(screen.queryByRole("button", { name: "Add new card" })).toBeNull();
  });
});

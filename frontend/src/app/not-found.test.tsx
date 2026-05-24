// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const mockBack = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ back: mockBack, push: vi.fn() }),
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
  it("renders the 404 heading, message, and navigation actions", () => {
    render(<NotFound />);

    expect(screen.getByText("404")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /page not found/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Go Back" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to home/i })).toHaveAttribute("href", "/");
  });

  it("'Go Back' calls router.back()", async () => {
    const user = userEvent.setup();
    render(<NotFound />);

    await user.click(screen.getByRole("button", { name: "Go Back" }));

    expect(mockBack).toHaveBeenCalledTimes(1);
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

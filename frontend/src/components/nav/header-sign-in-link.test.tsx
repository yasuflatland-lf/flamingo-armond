// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    children,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { children?: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
}));

const mockUsePathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => mockUsePathname(),
}));

import { HeaderSignInLink } from "./header-sign-in-link";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<HeaderSignInLink>", () => {
  describe("when on /login", () => {
    beforeEach(() => {
      mockUsePathname.mockReturnValue("/login");
    });

    it("renders nothing", () => {
      const { container } = render(<HeaderSignInLink className="foo" />);
      expect(container.firstChild).toBeNull();
      expect(screen.queryByRole("link")).not.toBeInTheDocument();
    });
  });

  describe("when on another path", () => {
    beforeEach(() => {
      mockUsePathname.mockReturnValue("/cardgroups");
    });

    it("renders a link to /login", () => {
      render(<HeaderSignInLink />);
      expect(screen.getByRole("link", { name: /sign in/i })).toHaveAttribute("href", "/login");
    });

    it("forwards className to the link", () => {
      render(<HeaderSignInLink className="text-sm underline md:hidden" />);
      expect(screen.getByRole("link", { name: /sign in/i })).toHaveClass(
        "text-sm",
        "underline",
        "md:hidden",
      );
    });
  });
});

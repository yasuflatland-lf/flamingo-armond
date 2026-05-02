// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Mock next/link so it renders a plain <a> in jsdom.
vi.mock("next/link", () => ({
  default: ({
    children,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { children?: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
}));

// usePathname is mocked per-test via vi.mocked().mockReturnValue.
const mockUsePathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => mockUsePathname(),
}));

import { HeaderAddCardLink } from "./header-add-card-link";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<HeaderAddCardLink>", () => {
  describe("when on /cards/new", () => {
    beforeEach(() => {
      mockUsePathname.mockReturnValue("/cards/new");
    });

    it("renders a link to /cards/new", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).toHaveAttribute("href", "/cards/new");
    });

    it("has aria-current='page'", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).toHaveAttribute("aria-current", "page");
    });

    it("has pointer-events-none class", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).toHaveClass("pointer-events-none");
    });

    it("has opacity-60 class", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).toHaveClass("opacity-60");
    });

    it("does not have hover:opacity-90 class", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).not.toHaveClass("hover:opacity-90");
    });
  });

  describe("when on a different path", () => {
    beforeEach(() => {
      mockUsePathname.mockReturnValue("/cardgroups");
    });

    it("renders a link to /cards/new", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).toHaveAttribute("href", "/cards/new");
    });

    it("does not have aria-current attribute", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).not.toHaveAttribute("aria-current");
    });

    it("does not have pointer-events-none class", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).not.toHaveClass("pointer-events-none");
    });

    it("does not have opacity-60 class", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).not.toHaveClass("opacity-60");
    });

    it("has hover:opacity-90 class", () => {
      render(<HeaderAddCardLink />);
      expect(screen.getByRole("link")).toHaveClass("hover:opacity-90");
    });
  });
});

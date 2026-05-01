// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { AdminSidebar } from "@/app/admin/_components/admin-sidebar";

// ---------------------------------------------------------------------------
// next/navigation mock — `redirect` throws so the layout aborts rendering the
// way Next.js's server runtime does. `usePathname` is set per-test via
// `vi.mocked(usePathname).mockReturnValue(...)`.
// ---------------------------------------------------------------------------

const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
  usePathname: vi.fn(() => "/admin/users"),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// Server-side dependencies for the layout.
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Pull mocked symbols *after* vi.mock calls register so vi.mocked() resolves them.
import { redirect, usePathname } from "next/navigation";
import AdminLayout from "@/app/admin/layout";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockSupabaseUser(user: { id: string } | null, error: Error | null = null) {
  vi.mocked(createSupabaseServerClient).mockResolvedValue({
    auth: {
      getUser: vi.fn().mockResolvedValue({
        data: { user },
        error,
      }),
    },
    // Server client surface used by gqlFetch is also mocked (gqlFetch itself
    // is fully mocked) so we do not need to provide getSession here.
  } as never);
}

function mockMeQuery(roles: { id: string; name: string }[]) {
  vi.mocked(gqlFetch).mockResolvedValue({
    me: { id: "u-1", roles },
  } as never);
}

function mockMeQueryError(message: string) {
  vi.mocked(gqlFetch).mockRejectedValue(new Error(message));
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ===========================================================================
// AdminSidebar — client component, active highlight via usePathname
// ===========================================================================

describe("AdminSidebar", () => {
  test("renders all three nav items: Users, Roles, Dictionary", () => {
    vi.mocked(usePathname).mockReturnValue("/admin/users");
    render(<AdminSidebar />);

    expect(screen.getByRole("link", { name: /users/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /roles/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /dictionary/i })).toBeInTheDocument();
  });

  test("highlights the active route via aria-current=page on exact match", () => {
    vi.mocked(usePathname).mockReturnValue("/admin/roles");
    render(<AdminSidebar />);

    const rolesLink = screen.getByRole("link", { name: /roles/i });
    expect(rolesLink).toHaveAttribute("aria-current", "page");

    const usersLink = screen.getByRole("link", { name: /users/i });
    expect(usersLink).not.toHaveAttribute("aria-current");
  });

  test("highlights the parent nav item for nested admin routes", () => {
    // /admin/roles/edit/123 must keep "Roles" active.
    vi.mocked(usePathname).mockReturnValue("/admin/roles/edit/123");
    render(<AdminSidebar />);

    expect(screen.getByRole("link", { name: /roles/i })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: /dictionary/i })).not.toHaveAttribute("aria-current");
  });

  test("does NOT highlight a nav item when its prefix is a substring but not a path segment", () => {
    // /admin/users-other should NOT match /admin/users — guarded by the
    // trailing slash in the prefix matcher.
    vi.mocked(usePathname).mockReturnValue("/admin/users-other");
    render(<AdminSidebar />);

    expect(screen.getByRole("link", { name: /users/i })).not.toHaveAttribute("aria-current");
  });
});

// ===========================================================================
// AdminLayout — server component executed directly via async invocation
// ===========================================================================

describe("AdminLayout (server component gate)", () => {
  test("redirects to / when no user is signed in", async () => {
    mockSupabaseUser(null);

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toThrow(
      `${REDIRECT_PREFIX}/`,
    );

    expect(redirect).toHaveBeenCalledWith("/");
    // gqlFetch must not run when the Supabase gate already failed.
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("rethrows when Supabase getUser surfaces a transport error", async () => {
    const oops = new Error("boom");
    mockSupabaseUser(null, oops);

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toBe(oops);

    expect(redirect).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("redirects to / when authenticated but the user has no admin role", async () => {
    mockSupabaseUser({ id: "u-1" });
    mockMeQuery([{ id: "r-general", name: "general" }]);

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toThrow(
      `${REDIRECT_PREFIX}/`,
    );

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("redirects to / when meData.me is null", async () => {
    mockSupabaseUser({ id: "u-1" });
    vi.mocked(gqlFetch).mockResolvedValue({ me: null } as never);

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toThrow(
      `${REDIRECT_PREFIX}/`,
    );

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("redirects to / when meData.me.roles is empty array", async () => {
    mockSupabaseUser({ id: "u-1" });
    vi.mocked(gqlFetch).mockResolvedValue({ me: { id: "u-1", roles: [] } } as never);

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toThrow(
      `${REDIRECT_PREFIX}/`,
    );

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("redirects to / when gqlFetch throws UNAUTHENTICATED", async () => {
    mockSupabaseUser({ id: "u-1" });
    mockMeQueryError('GraphQL errors: [{"message":"UNAUTHENTICATED: token expired"}]');

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toThrow(
      `${REDIRECT_PREFIX}/`,
    );

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("redirects to / when gqlFetch throws FORBIDDEN", async () => {
    mockSupabaseUser({ id: "u-1" });
    mockMeQueryError('GraphQL errors: [{"message":"FORBIDDEN: admin only"}]');

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toThrow(
      `${REDIRECT_PREFIX}/`,
    );

    expect(redirect).toHaveBeenCalledWith("/");
  });

  test("rethrows non-auth gqlFetch errors so the error boundary handles them", async () => {
    mockSupabaseUser({ id: "u-1" });
    const oops = new Error("network down");
    vi.mocked(gqlFetch).mockRejectedValue(oops);

    await expect(AdminLayout({ children: <div>child</div> })).rejects.toBe(oops);

    expect(redirect).not.toHaveBeenCalled();
  });

  test("renders sidebar + children when the user is admin", async () => {
    mockSupabaseUser({ id: "u-1" });
    mockMeQuery([{ id: "r-admin", name: "admin" }]);
    vi.mocked(usePathname).mockReturnValue("/admin/users");

    const tree = await AdminLayout({ children: <div>admin-child-marker</div> });
    const { container } = render(tree as React.ReactElement);

    expect(container.querySelector("aside")).toBeInTheDocument();
    expect(container.querySelector("main")).toBeInTheDocument();
    expect(screen.getByText("admin-child-marker")).toBeInTheDocument();
    // Sidebar nav present.
    expect(screen.getByRole("link", { name: /users/i })).toBeInTheDocument();

    expect(redirect).not.toHaveBeenCalled();
  });
});

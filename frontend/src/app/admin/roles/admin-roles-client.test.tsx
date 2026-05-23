// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AdminDeleteRoleDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import { AdminRolesClient, type RoleItem } from "./admin-roles-client";

// ---------------------------------------------------------------------------
// sonner mock — capture Undo action callback.
// ---------------------------------------------------------------------------
let lastToastLabel: string | undefined;
let lastUndoAction: (() => void) | undefined;
vi.mock("sonner", () => ({
  toast: vi.fn((label: string, opts?: { action?: { onClick?: () => void } }) => {
    lastToastLabel = label;
    lastUndoAction = opts?.action?.onClick;
    return "toast-id";
  }),
  Toaster: () => null,
}));

// usePathname is used by UndoDeleteProvider for flush-on-navigation.
vi.mock("next/navigation", () => ({
  usePathname: () => "/admin/roles",
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// SwipeableRow is used by RoleListItem for editable rows. Mock it as a plain
// div so tests are free of gesture library internals.
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function SwipeableRowMock(
      {
        children,
        disabled,
        onDelete: _onDelete,
        ariaLabel: _ariaLabel,
      }: {
        children: React.ReactNode;
        disabled?: boolean;
        onDelete: () => void;
        ariaLabel: string | null;
      },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return (
        <div data-testid="swipeable-row-mock" data-disabled={disabled ? "true" : "false"}>
          {children}
        </div>
      );
    }),
  };
});

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const SYSTEM_ROLES: RoleItem[] = [
  { id: "r-admin", name: "admin" },
  { id: "r-general", name: "general" },
];
const CUSTOM_ROLE: RoleItem = { id: "r-mod", name: "moderator" };

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderRoles(roles: RoleItem[], mocks: unknown[] = []) {
  render(
    <MockedProvider mocks={mocks as never}>
      <UndoDeleteProvider>
        <AdminRolesClient initialRoles={roles} />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

beforeEach(() => {
  lastToastLabel = undefined;
  lastUndoAction = undefined;
});

afterEach(() => {
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Original tests
// ---------------------------------------------------------------------------

describe("AdminRolesClient", () => {
  it("wires the ListingPageShell with title, description, and New role CTA", () => {
    render(
      <MockedProvider mocks={[]}>
        <UndoDeleteProvider>
          <AdminRolesClient initialRoles={[]} />
        </UndoDeleteProvider>
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Roles" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.getByText("Manage roles available to assign to users.")).toBeInTheDocument();
    // Button uses asChild + <Link>: the data-testid is forwarded to the
    // rendered <a>, so the testid handle IS the anchor element itself.
    const cta = screen.getByTestId("admin-roles-new-btn");
    expect(cta.tagName).toBe("A");
    expect(cta).toHaveAttribute("href", "/admin/roles/new");
  });

  it("renders editable rows as <Link> to the edit page", () => {
    renderRoles([CUSTOM_ROLE]);

    const link = screen.getByRole("link", { name: /moderator/i });
    expect(link).toHaveAttribute("href", `/admin/roles/${CUSTOM_ROLE.id}/edit`);
  });

  it("renders system rows without an edit link and labels them as system", () => {
    renderRoles(SYSTEM_ROLES);

    // No anchor for system roles: the row name is plain content.
    expect(screen.queryByRole("link", { name: /admin/i })).toBeNull();
    expect(screen.queryByRole("link", { name: /general/i })).toBeNull();
    // The row carries an explicit "System role" tag.
    expect(screen.getAllByText("System role")).toHaveLength(SYSTEM_ROLES.length);
  });

  it("removes a custom role from the list when delete succeeds", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const mocks = [
      {
        request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { deleteRole: true } },
      },
    ];

    renderRoles([CUSTOM_ROLE], mocks);

    await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

    // The optimistic removal hides the row immediately.
    await waitFor(() => {
      expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
    });

    // Advance past the undo window so the mutation is committed.
    vi.advanceTimersByTime(5100);
    vi.useRealTimers();
    await waitFor(() => {});
  });
});

// ---------------------------------------------------------------------------
// Spec § Testing: SwipeableRow wrapping
//
// "verify system roles are NOT wrapped in SwipeableRow"
// "editable role → SwipeableRow wrap"
// ---------------------------------------------------------------------------

describe("AdminRolesClient — SwipeableRow wrapping", () => {
  it("editable role rows are wrapped in SwipeableRow", () => {
    // The mock above renders SwipeableRow as data-testid="swipeable-row-mock".
    renderRoles([CUSTOM_ROLE]);

    expect(screen.getByTestId("swipeable-row-mock")).toBeInTheDocument();
  });

  it("system role rows are NOT wrapped in SwipeableRow", () => {
    renderRoles(SYSTEM_ROLES);

    // No SwipeableRow present for system roles.
    expect(screen.queryByTestId("swipeable-row-mock")).not.toBeInTheDocument();
  });

  it("renders both SwipeableRow and non-SwipeableRow when list mixes editable and system roles", () => {
    renderRoles([CUSTOM_ROLE, ...SYSTEM_ROLES]);

    // Only the one editable role is wrapped.
    const rows = screen.getAllByTestId("swipeable-row-mock");
    expect(rows).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// Spec § Testing: scheduleDelete invocation
// ---------------------------------------------------------------------------

describe("AdminRolesClient — scheduleDelete invocation", () => {
  it("calls scheduleDelete (shows toast) when the delete button is clicked", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const mocks = [
      {
        request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { deleteRole: true } },
      },
    ];

    renderRoles([CUSTOM_ROLE], mocks);

    await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

    // scheduleDelete invokes sonner toast — the mock captures the label.
    expect(lastToastLabel).toBe(`Role "${CUSTOM_ROLE.name}" deleted`);
    // An Undo callback must be registered.
    expect(lastUndoAction).toBeInstanceOf(Function);

    // Advance past the undo window to consume the pending mutation mock.
    vi.advanceTimersByTime(5100);
    vi.useRealTimers();
    await waitFor(() => {});
  });

  it("optimistically removes the role from the list on delete", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const mocks = [
      {
        request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { deleteRole: true } },
      },
    ];

    renderRoles([CUSTOM_ROLE], mocks);

    // Role row is visible before delete.
    expect(screen.getByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeInTheDocument();

    await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

    // Optimistic removal: row is gone immediately.
    await waitFor(() => {
      expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).not.toBeInTheDocument();
    });

    // Advance past the undo window to consume the pending mutation mock.
    vi.advanceTimersByTime(5100);
    vi.useRealTimers();
    await waitFor(() => {});
  });

  it("restores the role row when Undo is invoked within the window", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    // No mutation mock: Undo cancels the commit so the mutation must NOT fire.
    renderRoles([CUSTOM_ROLE]);

    await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

    // Row is gone optimistically.
    await waitFor(() => {
      expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).not.toBeInTheDocument();
    });

    // Invoke Undo within the 5-second window — rollback restores the row.
    expect(lastUndoAction).toBeInstanceOf(Function);
    lastUndoAction?.();

    await waitFor(() => {
      expect(screen.getByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeInTheDocument();
    });

    vi.useRealTimers();
  });
});

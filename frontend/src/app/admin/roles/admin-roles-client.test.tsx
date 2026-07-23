// @vitest-environment happy-dom

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  AdminCreateRoleDocument,
  AdminDeleteRoleDocument,
  AdminRoleDocument,
  AdminUpdateRoleDocument,
} from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import { renderWithIntl } from "@/test/render-with-intl";
import enMessages from "../../../../messages/en.json";
import jaMessages from "../../../../messages/ja.json";
import { AdminRolesClient, type RoleItem } from "./admin-roles-client";

// ---------------------------------------------------------------------------
// Runtime mocks
// ---------------------------------------------------------------------------

// sonner mock — captures the undo-toast label so the localized copy can be
// asserted. Without a mounted <Toaster> the real toast() is a no-op, so this
// is the only seam that observes the label.
let lastToastLabel: string | undefined;
vi.mock("sonner", () => ({
  toast: vi.fn((label: string) => {
    lastToastLabel = label;
    return "toast-id";
  }),
  Toaster: () => null,
}));

let mockPathname = "/admin/roles";
let mockSearchParamsValue = "";
const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
    refresh: mockRefresh,
  }),
  useSearchParams: () => new URLSearchParams(mockSearchParamsValue),
}));

// SwipeableRow is used by RoleListItem for editable rows. Mock it as a plain
// div so tests stay focused on our navigation and delete behavior.
vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function SwipeableRowMock(
      {
        children,
        disabled,
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
const ADMIN_ROLE: RoleItem = { id: "r-admin", name: "admin" };

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderRoles(
  roles: RoleItem[],
  mocks: unknown[] = [],
  intl?: { locale: "en" | "ja"; messages: typeof enMessages },
) {
  renderWithIntl(
    <MockedProvider mocks={mocks as never}>
      <UndoDeleteProvider>
        <AdminRolesClient initialRoles={roles} />
      </UndoDeleteProvider>
    </MockedProvider>,
    intl,
  );
}

/**
 * Substitutes a single ICU `{name}` argument in a raw catalog string, so the
 * undo-toast assertions stay sourced from `messages/*.json`.
 */
function withName(template: string, name: string) {
  return template.replace("{name}", name);
}

function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockPathname = "/admin/roles";
  mockSearchParamsValue = "";
  mockPush.mockReset();
  mockReplace.mockReset();
  mockRefresh.mockReset();
  lastToastLabel = undefined;
});

afterEach(() => {
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Listing behavior and row navigation
// ---------------------------------------------------------------------------

describe("AdminRolesClient", () => {
  it("wires the ListingPageShell with title and New role CTA", () => {
    renderWithIntl(
      <MockedProvider mocks={[]}>
        <UndoDeleteProvider>
          <AdminRolesClient initialRoles={[]} />
        </UndoDeleteProvider>
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Roles" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();
  });

  it("New role button opens ?new=true", async () => {
    const user = userEvent.setup();

    renderRoles([]);

    // With an empty list two "New role" buttons are rendered (header + empty-state CTA).
    // Use the testid to target the header button specifically.
    await user.click(screen.getByTestId("admin-roles-new-btn"));

    expect(mockPush).toHaveBeenCalledWith("/admin/roles?new=true", { scroll: false });
  });

  it("header 'New role' button is desktop-only (hidden md:inline-flex)", () => {
    renderRoles([CUSTOM_ROLE]);

    const btn = screen.getByTestId("admin-roles-new-btn");
    expect(btn.className).toContain("hidden");
    expect(btn.className).toContain("md:inline-flex");
  });

  describe("empty state", () => {
    it("renders the empty-state block when roles list is empty", () => {
      renderRoles([]);

      expect(screen.getByTestId("admin-roles-empty")).toBeInTheDocument();
      expect(screen.getByTestId("admin-roles-empty-cta")).toBeInTheDocument();
      expect(screen.getByText("No roles yet")).toBeInTheDocument();
    });

    it("does NOT render the empty-state block when roles exist", () => {
      renderRoles([CUSTOM_ROLE]);

      expect(screen.queryByTestId("admin-roles-empty")).toBeNull();
    });

    it("empty-state CTA opens ?new=true", async () => {
      const user = userEvent.setup();
      renderRoles([]);

      await user.click(screen.getByTestId("admin-roles-empty-cta"));

      expect(mockPush).toHaveBeenCalledWith("/admin/roles?new=true", { scroll: false });
    });
  });

  it("editable rows open ?edit=<id>", async () => {
    const user = userEvent.setup();

    renderRoles([CUSTOM_ROLE]);

    await user.click(screen.getByRole("button", { name: /edit role moderator/i }));

    expect(mockPush).toHaveBeenCalledWith(`/admin/roles?edit=${CUSTOM_ROLE.id}`, {
      scroll: false,
    });
  });

  it("renders system rows without edit affordances and labels them as system", () => {
    renderRoles(SYSTEM_ROLES);

    expect(screen.queryByRole("button", { name: /admin/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /general/i })).toBeNull();
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

    // Regression pin: the role row passes no className, so the mobile tap
    // guard must arrive from HoverRevealDeleteButton's base — without it an
    // invisible Delete button below the sm breakpoint fires this optimistic
    // delete on a stray tap. Tailwind CSS is not compiled under vitest, so the
    // class has no computed-style effect here and the click below still lands.
    const deleteBtn = screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`);
    expect(deleteBtn.className).toContain("pointer-events-none");
    expect(deleteBtn.className).toContain("motion-reduce:pointer-events-auto");

    await user.click(deleteBtn);

    await waitFor(() => {
      expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
    });

    vi.advanceTimersByTime(5100);
    vi.useRealTimers();
    await waitFor(() => {});
  });
});

describe("AdminRolesClient — SwipeableRow wrapping", () => {
  it("editable role rows are wrapped in SwipeableRow", () => {
    renderRoles([CUSTOM_ROLE]);

    expect(screen.getByTestId("swipeable-row-mock")).toBeInTheDocument();
  });

  it("system role rows are NOT wrapped in SwipeableRow", () => {
    renderRoles(SYSTEM_ROLES);

    expect(screen.queryByTestId("swipeable-row-mock")).not.toBeInTheDocument();
  });
});

describe("AdminRolesClient — create role sheet", () => {
  it("CreateRoleSuccess submits a normalized name and closes the sheet", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "new=true";

    const mutationCalled = vi.fn();
    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => {
          mutationCalled();
          return {
            data: {
              createRole: {
                __typename: "CreateRoleSuccess" as const,
                role: { __typename: "Role" as const, id: "r-new", name: "moderator" },
              },
            },
          };
        },
      },
    ];

    renderRoles([], mocks);

    await user.type(screen.getByRole("textbox"), "  Moderator  ");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
    expect(mockReplace).toHaveBeenCalledWith("/admin/roles", { scroll: false });
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("InputValidationError shows the typed validation banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "new=true";

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "admin" } },
        result: () => ({
          data: {
            createRole: {
              __typename: "InputValidationError" as const,
              field: "name",
              message: "role name already exists",
            },
          },
        }),
      },
    ];

    renderRoles([], mocks);

    await user.type(screen.getByRole("textbox"), "admin");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-new-validation-error")).toBeInTheDocument();
    });
    expect(screen.getByTestId("admin-role-new-validation-error")).toHaveTextContent(
      "role name already exists",
    );
    expect(mockReplace).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("clears the validation banner as the user edits the field (onEdit wiring)", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "new=true";

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "admin" } },
        result: () => ({
          data: {
            createRole: {
              __typename: "InputValidationError" as const,
              field: "name",
              message: "role name already exists",
            },
          },
        }),
      },
    ];

    renderRoles([], mocks);

    await user.type(screen.getByRole("textbox"), "admin");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-new-validation-error")).toBeInTheDocument();
    });

    // Editing the field re-homes the former onInput clear via RoleForm's onEdit.
    await user.type(screen.getByRole("textbox"), "x");
    await waitFor(() => {
      expect(screen.queryByTestId("admin-role-new-validation-error")).not.toBeInTheDocument();
    });
  });

  it.each([
    ["UNAUTHENTICATED", "session has expired", "admin-role-new-auth-error"],
    ["FORBIDDEN", "do not have permission", "admin-role-new-auth-error"],
  ] as const)("auth rejection %s shows the login-link banner", async (code, message, testId) => {
    const user = userEvent.setup();
    mockSearchParamsValue = "new=true";

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        error: makeCodedError(code),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderRoles([], mocks);

      await user.type(screen.getByRole("textbox"), "moderator");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => {
        expect(screen.getByTestId(testId)).toBeInTheDocument();
      });
      expect(screen.getByTestId(testId)).toHaveTextContent(message);
      const banner = screen.getByTestId(testId);
      expect(within(banner).getByRole("link", { name: /sign in again/i })).toHaveAttribute(
        "href",
        "/login",
      );
      expect(mockReplace).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("unexpected __typename warns and shows degraded banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "new=true";

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => ({
          data: {
            createRole: {
              __typename: "FutureVariantClientDidNotKnowAbout",
            } as never,
          },
        }),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderRoles([], mocks);

      await user.type(screen.getByRole("textbox"), "moderator");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toBeInTheDocument();
      expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toHaveTextContent(
        /something went wrong/i,
      );
      expect(mockReplace).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected createRole payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("null createRole payload warns and shows degraded banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "new=true";

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => ({
          data: { createRole: null as never },
        }),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderRoles([], mocks);

      await user.type(screen.getByRole("textbox"), "moderator");
      await user.click(screen.getByRole("button", { name: /create/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toBeInTheDocument();
      expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toHaveTextContent(
        /something went wrong/i,
      );
      expect(mockReplace).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected createRole payload"),
        expect.objectContaining({ typename: null }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });
});

describe("AdminRolesClient — edit role sheet", () => {
  it("loads the full role record and saves through the sheet", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=r-mod";

    const mutationCalled = vi.fn();
    const mocks = [
      {
        request: { query: AdminRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { role: { __typename: "Role" as const, ...CUSTOM_ROLE } } },
      },
      {
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        result: () => {
          mutationCalled();
          return {
            data: {
              updateRole: {
                __typename: "UpdateRoleSuccess" as const,
                role: { __typename: "Role" as const, id: CUSTOM_ROLE.id, name: "reviewer" },
              },
            },
          };
        },
      },
    ];

    renderRoles([CUSTOM_ROLE], mocks);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /edit role/i })).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole("textbox")).toHaveValue("moderator");
    });

    await user.clear(screen.getByRole("textbox"));
    await user.type(screen.getByRole("textbox"), "Reviewer");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
    expect(mockReplace).toHaveBeenCalledWith("/admin/roles", { scroll: false });
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("renders the system-role banner and disables submit when role is admin", async () => {
    mockSearchParamsValue = "edit=r-admin";

    const mocks = [
      {
        request: { query: AdminRoleDocument, variables: { id: ADMIN_ROLE.id } },
        result: { data: { role: { __typename: "Role" as const, ...ADMIN_ROLE } } },
      },
    ];

    renderRoles([ADMIN_ROLE], mocks);

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-edit-system-banner")).toBeInTheDocument();
    });
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });

  it("CannotModifySystemRoleError response shows the typed banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=r-mod";

    const mocks = [
      {
        request: { query: AdminRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { role: { __typename: "Role" as const, ...CUSTOM_ROLE } } },
      },
      {
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        result: () => ({
          data: {
            updateRole: {
              __typename: "CannotModifySystemRoleError" as const,
              message: 'cannot rename system role "admin"',
              roleId: CUSTOM_ROLE.id,
              roleName: "admin",
            },
          },
        }),
      },
    ];

    renderRoles([CUSTOM_ROLE], mocks);

    await waitFor(() => {
      expect(screen.getByRole("textbox")).toHaveValue("moderator");
    });
    await user.clear(screen.getByRole("textbox"));
    await user.type(screen.getByRole("textbox"), "Reviewer");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-edit-system-role-error")).toBeInTheDocument();
    });
    expect(screen.getByTestId("admin-role-edit-system-role-error")).toHaveTextContent(
      'cannot rename system role "admin"',
    );
    expect(mockReplace).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it.each([
    ["UNAUTHENTICATED", "session has expired", "admin-role-edit-auth-error"],
    ["FORBIDDEN", "do not have permission", "admin-role-edit-auth-error"],
  ] as const)("auth rejection %s shows the login-link banner", async (code, message, testId) => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=r-mod";

    const mocks = [
      {
        request: { query: AdminRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { role: { __typename: "Role" as const, ...CUSTOM_ROLE } } },
      },
      {
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        error: makeCodedError(code),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderRoles([CUSTOM_ROLE], mocks);

      await waitFor(() => {
        expect(screen.getByRole("textbox")).toHaveValue("moderator");
      });
      await user.clear(screen.getByRole("textbox"));
      await user.type(screen.getByRole("textbox"), "Reviewer");
      await user.click(screen.getByRole("button", { name: /save/i }));

      await waitFor(() => {
        expect(screen.getByTestId(testId)).toBeInTheDocument();
      });
      expect(screen.getByTestId(testId)).toHaveTextContent(message);
      const banner = screen.getByTestId(testId);
      expect(within(banner).getByRole("link", { name: /sign in again/i })).toHaveAttribute(
        "href",
        "/login",
      );
      expect(mockReplace).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("unexpected __typename warns and shows degraded banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=r-mod";

    const mocks = [
      {
        request: { query: AdminRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { role: { __typename: "Role" as const, ...CUSTOM_ROLE } } },
      },
      {
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        result: () => ({
          data: {
            updateRole: {
              __typename: "FutureVariantClientDidNotKnowAbout",
            } as never,
          },
        }),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderRoles([CUSTOM_ROLE], mocks);

      await waitFor(() => {
        expect(screen.getByRole("textbox")).toHaveValue("moderator");
      });
      await user.clear(screen.getByRole("textbox"));
      await user.type(screen.getByRole("textbox"), "Reviewer");
      await user.click(screen.getByRole("button", { name: /save/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByTestId("admin-role-edit-unexpected-payload-error")).toBeInTheDocument();
      expect(screen.getByTestId("admin-role-edit-unexpected-payload-error")).toHaveTextContent(
        /something went wrong/i,
      );
      expect(mockReplace).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected updateRole payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("null updateRole payload warns and shows degraded banner", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=r-mod";

    const mocks = [
      {
        request: { query: AdminRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { role: { __typename: "Role" as const, ...CUSTOM_ROLE } } },
      },
      {
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        result: () => ({
          data: { updateRole: null as never },
        }),
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      renderRoles([CUSTOM_ROLE], mocks);

      await waitFor(() => {
        expect(screen.getByRole("textbox")).toHaveValue("moderator");
      });
      await user.clear(screen.getByRole("textbox"));
      await user.type(screen.getByRole("textbox"), "Reviewer");
      await user.click(screen.getByRole("button", { name: /save/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      expect(screen.getByTestId("admin-role-edit-unexpected-payload-error")).toBeInTheDocument();
      expect(screen.getByTestId("admin-role-edit-unexpected-payload-error")).toHaveTextContent(
        /something went wrong/i,
      );
      expect(mockReplace).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected updateRole payload"),
        expect.objectContaining({ typename: null }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });
});

// ---------------------------------------------------------------------------
// AdminRolesClient — undo-toast label (localized copy + {name} argument)
// ---------------------------------------------------------------------------

describe("AdminRolesClient — undo-toast label", () => {
  it.each([
    ["en" as const, enMessages],
    ["ja" as const, jaMessages],
  ])("renders the %s undo-toast label with the {name} argument", async (locale, messages) => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const mocks = [
      {
        request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { adminDeleteRole: true } },
      },
    ];
    renderRoles([CUSTOM_ROLE], mocks, { locale, messages });

    await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

    expect(lastToastLabel).toBe(withName(messages.Admin.roleDeleted, CUSTOM_ROLE.name));

    act(() => vi.advanceTimersByTime(5100));
    vi.useRealTimers();
    await waitFor(() => {});
  });
});

// ---------------------------------------------------------------------------
// AdminRolesClient — delete failure (onCommitFailed path)
// ---------------------------------------------------------------------------

describe("AdminRolesClient — delete failure", () => {
  it("shows error banner and restores row when commit fails with FORBIDDEN", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      const mocks = [
        {
          request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
          result: {
            errors: [
              new GraphQLError("Forbidden", {
                extensions: { code: "FORBIDDEN" },
              }),
            ],
          },
        },
      ];

      // Start from a non-empty list so the list view renders initially.
      renderRoles([CUSTOM_ROLE], mocks);

      // Trigger optimistic delete — row disappears immediately.
      await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

      await waitFor(() => {
        expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
      });

      // Advance past the 5-second undo window so commitDelete fires and fails.
      act(() => vi.advanceTimersByTime(5100));
      vi.useRealTimers();

      // optimisticRollback restores the row.
      await waitFor(() => {
        expect(screen.getByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeInTheDocument();
      });

      // Error banner is visible inside the list.
      expect(screen.getByTestId("admin-roles-error")).toBeInTheDocument();

      // The list container is shown (not the empty-state) because deleteError is set.
      expect(screen.getByTestId("admin-roles-list")).toBeInTheDocument();
      expect(screen.queryByTestId("admin-roles-empty")).toBeNull();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("shows error banner (not empty-state) when the last role is deleted and commit fails", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      const mocks = [
        {
          request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
          result: {
            errors: [
              new GraphQLError("Forbidden", {
                extensions: { code: "FORBIDDEN" },
              }),
            ],
          },
        },
      ];

      // Single custom (deletable) role — deleting it empties `roles` optimistically.
      renderRoles([CUSTOM_ROLE], mocks);

      await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

      // Optimistic removal empties the list; empty-state appears briefly
      // (roles.length === 0 && !deleteError is true at this point).
      await waitFor(() => {
        expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
      });

      // Advance past the undo window — commit fires, fails, rollback restores the role,
      // and deleteError is set.
      act(() => vi.advanceTimersByTime(5100));
      vi.useRealTimers();

      // After rollback: the deleted role is restored (roles.length === 1), so the
      // list is non-empty and the empty-state condition is false regardless of
      // deleteError. The error banner renders inside the (non-empty) list.
      await waitFor(() => {
        expect(screen.getByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeInTheDocument();
      });
      expect(screen.queryByTestId("admin-roles-empty")).toBeNull();
      expect(screen.getByTestId("admin-roles-error")).toBeInTheDocument();
    } finally {
      warnSpy.mockRestore();
    }
  });

  // Regression locks for the deliberate divergence from the shared kind-only
  // classifyMutationAuthError used by the create/update branches: delete passes
  // the server's FORBIDDEN message through verbatim (system-role protection) but
  // collapses UNAUTHENTICATED to generic copy. A future "DRY" refactor that
  // routed delete through the kind-only classifier would break these.
  it("keeps the server's FORBIDDEN message verbatim when deleting a system role is refused", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      const mocks = [
        {
          request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
          result: {
            errors: [
              new GraphQLError('cannot delete system role "admin"', {
                extensions: { code: "FORBIDDEN" },
              }),
            ],
          },
        },
      ];

      renderRoles([CUSTOM_ROLE], mocks);

      await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));
      await waitFor(() => {
        expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
      });

      act(() => vi.advanceTimersByTime(5100));
      vi.useRealTimers();

      await waitFor(() => {
        expect(screen.getByTestId("admin-roles-error")).toHaveTextContent(
          'cannot delete system role "admin"',
        );
      });
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("collapses a delete UNAUTHENTICATED failure to the generic sign-in copy, dropping the server detail", async () => {
    const user = userEvent.setup();
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      const mocks = [
        {
          request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
          result: {
            errors: [
              new GraphQLError("token expired at the edge proxy", {
                extensions: { code: "UNAUTHENTICATED" },
              }),
            ],
          },
        },
      ];

      renderRoles([CUSTOM_ROLE], mocks);

      await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));
      await waitFor(() => {
        expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
      });

      act(() => vi.advanceTimersByTime(5100));
      vi.useRealTimers();

      const banner = await screen.findByTestId("admin-roles-error");
      expect(banner).toHaveTextContent("Your session has expired. Please sign in again.");
      expect(banner).not.toHaveTextContent("token expired at the edge proxy");
    } finally {
      warnSpy.mockRestore();
    }
  });
});

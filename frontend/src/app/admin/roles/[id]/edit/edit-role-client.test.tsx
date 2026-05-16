// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AdminUpdateRoleDocument } from "@/generated/graphql";
import { EditRoleClient, type RoleForEdit } from "./edit-role-client";

const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const CUSTOM_ROLE: RoleForEdit = { id: "r-mod", name: "moderator" };
const ADMIN_ROLE: RoleForEdit = { id: "r-admin", name: "admin" };

describe("EditRoleClient", () => {
  it("renders heading + cancel link", () => {
    render(
      <MockedProvider mocks={[]}>
        <EditRoleClient role={CUSTOM_ROLE} />
      </MockedProvider>,
    );
    expect(screen.getByRole("heading", { name: /edit role/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /cancel/i })).toHaveAttribute("href", "/admin/roles");
  });

  it("UpdateRoleSuccess response — navigates and refreshes", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const mutationCalled = vi.fn();
    const mocks = [
      {
        // Zod transforms input to trim+lowercase before submit, so the
        // mutation sees "reviewer" regardless of mixed-case typed input.
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

    render(
      <MockedProvider mocks={mocks}>
        <EditRoleClient role={CUSTOM_ROLE} />
      </MockedProvider>,
    );

    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "Reviewer");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
    expect(mockPush).toHaveBeenCalledWith("/admin/roles");
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("CannotModifySystemRoleError response — shows banner, no navigation, no cache mutation", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const mocks = [
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

    render(
      <MockedProvider mocks={mocks}>
        <EditRoleClient role={CUSTOM_ROLE} />
      </MockedProvider>,
    );

    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "Reviewer");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-edit-system-role-error")).toBeInTheDocument();
    });
    expect(screen.getByTestId("admin-role-edit-system-role-error")).toHaveTextContent(
      'cannot rename system role "admin"',
    );
    // No navigation: the error variant is data, not a success.
    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("renders the system-role banner and disables submit when role is admin", () => {
    render(
      <MockedProvider mocks={[]}>
        <EditRoleClient role={ADMIN_ROLE} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-role-edit-system-banner")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });

  it("transport rejection — no banner, no navigation, console.warn omits err.message", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const networkError = new Error("network down");
    const mocks = [
      {
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        error: networkError,
      },
    ];

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      render(
        <MockedProvider mocks={mocks}>
          <EditRoleClient role={CUSTOM_ROLE} />
        </MockedProvider>,
      );

      const input = screen.getByRole("textbox");
      await user.clear(input);
      await user.type(input, "Reviewer");
      await user.click(screen.getByRole("button", { name: /save/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      // No navigation on transport failure.
      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      // No typed-error banner.
      expect(
        screen.queryByTestId("admin-role-edit-system-role-error"),
      ).not.toBeInTheDocument();

      // Warn payload MUST NOT include err.message — backend messages may echo user input.
      // Per .claude/rules/frontend-rsc-error-handling.md § "Redact err.message from structured console payloads".
      expect(warnSpy).not.toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({ message: expect.anything() }),
      );
      // Warn payload MUST include err.name — proves production code logs name, not message.
      expect(warnSpy).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ name: "Error" }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("unexpected __typename — warns and shows degraded banner, no navigation", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const mocks = [
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
      render(
        <MockedProvider mocks={mocks}>
          <EditRoleClient role={CUSTOM_ROLE} />
        </MockedProvider>,
      );

      const input = screen.getByRole("textbox");
      await user.clear(input);
      await user.type(input, "Reviewer");
      await user.click(screen.getByRole("button", { name: /save/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      // Degraded banner is shown.
      expect(screen.getByTestId("admin-role-edit-system-role-error")).toBeInTheDocument();
      expect(screen.getByTestId("admin-role-edit-system-role-error")).toHaveTextContent(
        /something went wrong/i,
      );

      // No navigation — this is not a success variant.
      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      // Warn payload includes the unexpected typename.
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected updateRole payload"),
        expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("null updateRole payload (partial-response null bubble) — warns and shows banner", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const mocks = [
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
      render(
        <MockedProvider mocks={mocks}>
          <EditRoleClient role={CUSTOM_ROLE} />
        </MockedProvider>,
      );

      const input = screen.getByRole("textbox");
      await user.clear(input);
      await user.type(input, "Reviewer");
      await user.click(screen.getByRole("button", { name: /save/i }));

      await waitFor(() => expect(warnSpy).toHaveBeenCalled());

      // Degraded banner is shown.
      expect(screen.getByTestId("admin-role-edit-system-role-error")).toBeInTheDocument();
      expect(screen.getByTestId("admin-role-edit-system-role-error")).toHaveTextContent(
        /something went wrong/i,
      );

      // No navigation.
      expect(mockPush).not.toHaveBeenCalled();
      expect(mockRefresh).not.toHaveBeenCalled();

      // typename is null in the warn payload for a null bubble.
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining("unexpected updateRole payload"),
        expect.objectContaining({ typename: null }),
      );
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("renders the system-role banner for the general role", () => {
    render(
      <MockedProvider mocks={[]}>
        <EditRoleClient role={{ id: "r-general", name: "general" }} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-role-edit-system-banner")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });
});

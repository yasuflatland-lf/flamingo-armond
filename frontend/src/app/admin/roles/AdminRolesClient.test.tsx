// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AdminRolesClient } from "./AdminRolesClient";

describe("AdminRolesClient", () => {
  it("renders the AdminPageShell heading", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminRolesClient initialRoles={[]} />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Roles" })).toBeInTheDocument();
  });
});

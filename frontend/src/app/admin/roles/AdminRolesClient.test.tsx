// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AdminRolesClient } from "./AdminRolesClient";

describe("AdminRolesClient", () => {
  it("wires the ListingPageShell with title and description", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminRolesClient initialRoles={[]} />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Roles" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.getByText("Manage roles available to assign to users.")).toBeInTheDocument();
  });
});

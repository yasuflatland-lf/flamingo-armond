// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { LoginButton } from "./login-button";

// The browser Supabase client is only constructed inside the click handler, but
// mock the module so importing the component never reaches the real client.
vi.mock("@/lib/supabase/client", () => ({
  createSupabaseBrowserClient: vi.fn(),
}));

describe("<LoginButton>", () => {
  it("renders the localized Google sign-in label", () => {
    // renderWithIntl defaults to the en catalog, so `t("googleButton")` resolves
    // to "Continue with Google" — this pins the Login.googleButton key.
    renderWithIntl(<LoginButton />);

    expect(screen.getByRole("button", { name: /continue with google/i })).toBeInTheDocument();
  });
});

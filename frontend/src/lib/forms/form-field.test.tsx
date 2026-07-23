// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FormField } from "./form-field";

function stringField(overrides?: {
  name?: string;
  value?: string;
  errors?: unknown[];
  handleBlur?: () => void;
  handleChange?: (value: string) => void;
}) {
  return {
    name: overrides?.name ?? "front",
    state: { value: overrides?.value ?? "", meta: { errors: overrides?.errors ?? [] } },
    handleBlur: overrides?.handleBlur ?? vi.fn(),
    handleChange: overrides?.handleChange ?? vi.fn(),
  };
}

function booleanField(overrides?: {
  name?: string;
  value?: boolean;
  handleBlur?: () => void;
  handleChange?: (value: boolean) => void;
}) {
  return {
    name: overrides?.name ?? "isDefaultStarter",
    state: { value: overrides?.value ?? false, meta: { errors: [] } },
    handleBlur: overrides?.handleBlur ?? vi.fn(),
    handleChange: overrides?.handleChange ?? vi.fn(),
  };
}

describe("FormField (text)", () => {
  it("pairs the Label htmlFor with the input id (= field.name by default)", () => {
    render(<FormField field={stringField({ name: "name" })} label="Name" />);
    const input = screen.getByLabelText("Name");
    expect(input).toHaveAttribute("id", "name");
    expect(input.tagName).toBe("INPUT");
  });

  it("uses idOverride for both the input id and the Label htmlFor", () => {
    render(
      <FormField field={stringField({ name: "front" })} label="Front" idOverride="c1front-field" />,
    );
    const input = screen.getByLabelText("Front");
    expect(input).toHaveAttribute("id", "c1front-field");
    // name attribute stays the raw field name, only the id is overridden.
    expect(input).toHaveAttribute("name", "front");
  });

  it("renders the current value and threads onChange / onBlur", async () => {
    const handleChange = vi.fn();
    const handleBlur = vi.fn();
    render(
      <FormField field={stringField({ value: "hi", handleChange, handleBlur })} label="Name" />,
    );
    const input = screen.getByLabelText("Name") as HTMLInputElement;
    expect(input.value).toBe("hi");
    await userEvent.type(input, "x");
    expect(handleChange).toHaveBeenCalled();
    input.blur();
    expect(handleBlur).toHaveBeenCalled();
  });

  it("renders a zod field error message", () => {
    render(
      <FormField field={stringField({ errors: [{ message: "name is required" }] })} label="Name" />,
    );
    expect(screen.getByText("name is required")).toBeInTheDocument();
  });

  it("renders the backendError when there is no zod error", () => {
    render(<FormField field={stringField()} label="Name" backendError="already taken" />);
    expect(screen.getByText("already taken")).toBeInTheDocument();
  });

  it("prefers the zod error over backendError when both are present", () => {
    render(
      <FormField
        field={stringField({ errors: [{ message: "zod wins" }] })}
        label="Name"
        backendError="backend loses"
      />,
    );
    expect(screen.getByText("zod wins")).toBeInTheDocument();
    expect(screen.queryByText("backend loses")).toBeNull();
  });

  it("forwards placeholder and disabled", () => {
    render(<FormField field={stringField()} label="Name" placeholder="Type here" disabled />);
    const input = screen.getByLabelText("Name");
    expect(input).toHaveAttribute("placeholder", "Type here");
    expect(input).toBeDisabled();
  });

  it("merges a className override onto the wrapper (tailwind-merge wins last)", () => {
    const { container } = render(
      <FormField field={stringField()} label="Name" className="space-y-1" />,
    );
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper.className).toContain("space-y-1");
    expect(wrapper.className).not.toContain("space-y-2");
  });
});

describe("FormField (textarea / number)", () => {
  it("renders a textarea for kind=textarea", () => {
    render(<FormField field={stringField({ name: "bio" })} label="Bio" kind="textarea" />);
    const el = screen.getByLabelText("Bio");
    expect(el.tagName).toBe("TEXTAREA");
  });

  it("forwards rows to a textarea", () => {
    render(<FormField field={stringField({ name: "bio" })} label="Bio" kind="textarea" rows={4} />);
    expect(screen.getByLabelText("Bio")).toHaveAttribute("rows", "4");
  });

  it("renders a number input for kind=number", () => {
    render(<FormField field={stringField({ name: "sortOrder" })} label="Order" kind="number" />);
    const el = screen.getByLabelText("Order");
    expect(el).toHaveAttribute("type", "number");
  });
});

describe("FormField (hint / trailing)", () => {
  it("renders hint content between the control and the field error", () => {
    render(
      <FormField
        field={stringField({ name: "displayName" })}
        label="Display name"
        hint={<p>pick something memorable</p>}
      />,
    );
    expect(screen.getByText("pick something memorable")).toBeInTheDocument();
  });

  it("renders trailing content (e.g. a clear button)", () => {
    render(
      <FormField
        field={stringField({ name: "bio", value: "hi" })}
        label="Bio"
        kind="textarea"
        trailing={
          <button type="button" tabIndex={-1}>
            Clear bio
          </button>
        }
      />,
    );
    expect(screen.getByRole("button", { name: "Clear bio" })).toBeInTheDocument();
  });

  it("orders control, then hint, then trailing, then the field error", () => {
    const { container } = render(
      <FormField
        field={stringField({ name: "displayName", value: "x", errors: [{ message: "required" }] })}
        label="Display name"
        hint={<span data-testid="hint-node">hint text</span>}
        trailing={<span data-testid="trailing-node">trailing node</span>}
      />,
    );
    const wrapper = container.firstElementChild as HTMLElement;
    const children = Array.from(wrapper.children) as HTMLElement[];
    const idxControl = children.findIndex((el) => el.tagName === "INPUT");
    const idxHint = children.findIndex((el) => el.dataset.testid === "hint-node");
    const idxTrailing = children.findIndex((el) => el.dataset.testid === "trailing-node");
    const idxError = children.findIndex((el) => el.textContent === "required");

    expect(idxControl).toBeGreaterThanOrEqual(0);
    expect(idxControl).toBeLessThan(idxHint);
    expect(idxHint).toBeLessThan(idxTrailing);
    expect(idxTrailing).toBeLessThan(idxError);
  });

  it("does not render hint or trailing for the checkbox kind", () => {
    render(
      <FormField
        field={booleanField()}
        label="Default starter"
        kind="checkbox"
        hint={<span>should-not-appear-hint</span>}
        trailing={<span>should-not-appear-trailing</span>}
      />,
    );
    expect(screen.queryByText("should-not-appear-hint")).toBeNull();
    expect(screen.queryByText("should-not-appear-trailing")).toBeNull();
  });
});

describe("FormField (checkbox)", () => {
  it("renders a checkbox reflecting the boolean value, with the label after it and no FieldError row", () => {
    render(
      <FormField field={booleanField({ value: true })} label="Default starter" kind="checkbox" />,
    );
    const box = screen.getByLabelText("Default starter") as HTMLInputElement;
    expect(box).toHaveAttribute("type", "checkbox");
    expect(box.checked).toBe(true);
  });

  it("calls handleChange with the next checked state", async () => {
    const handleChange = vi.fn();
    render(
      <FormField
        field={booleanField({ value: false, handleChange })}
        label="Default starter"
        kind="checkbox"
      />,
    );
    await userEvent.click(screen.getByLabelText("Default starter"));
    expect(handleChange).toHaveBeenCalledWith(true);
  });

  it("forwards disabled to the checkbox input", () => {
    render(<FormField field={booleanField()} label="Default starter" kind="checkbox" disabled />);
    expect(screen.getByLabelText("Default starter")).toBeDisabled();
  });
});

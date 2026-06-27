import type { ReactNode } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { FieldError } from "@/lib/forms/field-error";
import { cn } from "@/lib/utils";

/** Minimal structural view of a TanStack field whose value is a string. */
type StringFieldApi = {
  name: string;
  state: { value: string; meta: { errors: unknown[] } };
  handleBlur: () => void;
  handleChange: (value: string) => void;
};

/** Minimal structural view of a TanStack field whose value is a boolean. */
type BooleanFieldApi = {
  name: string;
  state: { value: boolean; meta: { errors: unknown[] } };
  handleBlur: () => void;
  handleChange: (value: boolean) => void;
};

type CommonProps = {
  /** Already-translated label text — the wrapper never calls `useTranslations`. */
  label: string;
  /** Resolved backend error message for this field, if any. */
  backendError?: string;
  /**
   * Overrides the control `id` (and the `Label` `htmlFor`). Defaults to
   * `field.name`. Required when several forms render the same field name on one
   * page — e.g. `CardForm`'s `${idPrefix}front-field`.
   */
  idOverride?: string;
  disabled?: boolean;
  placeholder?: string;
  /** Forwarded to the rendered control as `data-testid`. */
  testId?: string;
  /** Extra classes for the field wrapper. Default spacing is `space-y-2`. */
  className?: string;
};

type FormFieldProps =
  | (CommonProps & { kind?: "text" | "textarea" | "number"; field: StringFieldApi })
  | (CommonProps & { kind: "checkbox"; field: BooleanFieldApi });

/**
 * The label + control + {@link FieldError} triad shared by the TanStack forms.
 *
 * It owns the `htmlFor`/`id` a11y pairing so the two can never drift, and threads
 * the standard `onBlur`/`onChange` wiring for the chosen control. Labels arrive
 * already-translated; the wrapper performs no i18n.
 *
 * `kind` selects the control: `"text"` (default) / `"number"` render an
 * {@link Input}, `"textarea"` renders a {@link Textarea}, and `"checkbox"`
 * renders an inline checkbox + label with no `FieldError` row.
 */
export function FormField(props: FormFieldProps) {
  const { label, backendError, idOverride, disabled, placeholder, testId, className } = props;
  const id = idOverride ?? props.field.name;

  if (props.kind === "checkbox") {
    const { field } = props;
    return (
      <div className={cn("flex items-center gap-2", className)}>
        <input
          id={id}
          name={field.name}
          type="checkbox"
          data-testid={testId}
          checked={field.state.value}
          onBlur={field.handleBlur}
          onChange={(e) => field.handleChange(e.target.checked)}
          disabled={disabled}
          className="h-4 w-4 rounded border-input"
        />
        <Label htmlFor={id}>{label}</Label>
      </div>
    );
  }

  const { field, kind } = props;
  const control: ReactNode =
    kind === "textarea" ? (
      <Textarea
        id={id}
        name={field.name}
        data-testid={testId}
        value={field.state.value}
        onBlur={field.handleBlur}
        onChange={(e) => field.handleChange(e.target.value)}
        disabled={disabled}
        placeholder={placeholder}
      />
    ) : (
      <Input
        id={id}
        name={field.name}
        type={kind === "number" ? "number" : undefined}
        data-testid={testId}
        value={field.state.value}
        onBlur={field.handleBlur}
        onChange={(e) => field.handleChange(e.target.value)}
        disabled={disabled}
        placeholder={placeholder}
      />
    );

  return (
    <div className={cn("space-y-2", className)}>
      <Label htmlFor={id}>{label}</Label>
      {control}
      <FieldError zodErrors={field.state.meta.errors} backendError={backendError} />
    </div>
  );
}

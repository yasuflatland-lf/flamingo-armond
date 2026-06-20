import type { FormEvent } from "react";

/** Minimal structural view of a TanStack form needed to submit it. */
type SubmittableForm = { handleSubmit: () => Promise<unknown> };

/**
 * Builds the `<form onSubmit>` handler shared by every TanStack form body.
 *
 * It cancels the native submit, then runs `form.handleSubmit()` and swallows
 * the returned promise's rejection. That rejection is expected: the form's
 * inner submit handler re-throws on failure — via {@link wrapSubmit} in the
 * simple forms, or its own outcome-switch handler in `profile-form` /
 * `onboarding-form` — so TanStack keeps `formState.isSubmitSuccessful` false.
 * In every case the inner handler has already surfaced and logged the failure,
 * so swallowing here only stops the re-thrown rejection from becoming an
 * unhandled browser promise rejection.
 */
export function submitFormHandler(form: SubmittableForm) {
  return (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    e.stopPropagation();
    form.handleSubmit().catch(() => {
      // Already logged by the inner submit handler; see the doc comment above.
    });
  };
}

/**
 * Wraps a form's async `submit` so a rejection is logged (redacted) and
 * re-thrown.
 *
 * The re-throw is load-bearing: it keeps TanStack's
 * `formState.isSubmitSuccessful` false on failure. The log omits `err.message`
 * because backend messages may echo user-authored input — see
 * docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
 *
 * @param scope console-prefix tag, e.g. `"cardgroup-form"`.
 * @param submit the parent-supplied mutation runner.
 */
export function wrapSubmit<T>(
  scope: string,
  submit: (value: T) => Promise<void>,
): (value: T) => Promise<void> {
  return (value: T) =>
    submit(value).catch((err: unknown) => {
      console.error(`[${scope}] submit rejected`, {
        name: err instanceof Error ? err.name : "unknown",
      });
      throw err; // keep formState.isSubmitSuccessful correct
    });
}

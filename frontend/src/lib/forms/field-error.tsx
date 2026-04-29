function hasMessage(value: unknown): value is { message: string } {
  return typeof (value as { message?: unknown })?.message === "string";
}

type FieldErrorProps = { zodErrors: unknown[]; backendError?: string };

export function FieldError({ zodErrors, backendError }: FieldErrorProps) {
  const msg = zodErrors.find(hasMessage)?.message ?? backendError;
  if (!msg) return null;
  return <p className="text-sm text-destructive">{msg}</p>;
}

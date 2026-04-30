"use client";

type Props = {
  completed: number;
  remaining: number;
  saving?: boolean;
};

export function SwipeStatusBar({ completed, remaining, saving = false }: Props) {
  return (
    <div className="mx-auto mb-4 flex w-full max-w-xl items-center justify-between rounded-lg border border-border bg-card px-4 py-3 text-sm">
      <span className="font-medium text-foreground">
        {completed} completed / {remaining} remaining
      </span>
      <span className="text-muted-foreground">{saving ? "Saving..." : "Ready"}</span>
    </div>
  );
}

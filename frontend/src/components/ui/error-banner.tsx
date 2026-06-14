import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

/**
 * Destructive message banner. Defaults role="alert"; pass extra layout classes
 * via className (merged with cn) and any DOM props (e.g. data-testid) via ...props.
 */
export function ErrorBanner({ className, children, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      role="alert"
      className={cn("rounded-md bg-destructive/10 p-3 text-sm text-destructive", className)}
      {...props}
    >
      {children}
    </div>
  );
}

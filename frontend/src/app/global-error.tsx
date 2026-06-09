"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";

type GlobalErrorProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

// Top-level error boundary that catches errors thrown from the root layout
// and any route segment without its own error.tsx. This component replaces
// the root layout when it renders, so it must include its own <html>/<body>.
export default function GlobalError({ error, reset }: GlobalErrorProps) {
  useEffect(() => {
    // Log with a scope prefix so this boundary is identifiable in production
    // log streams.
    console.error("[global-error]", {
      message: error.message,
      digest: error.digest,
    });
  }, [error]);

  return (
    <html lang="en">
      <body>
        <main className="flex min-h-dvh w-full flex-col items-center justify-center gap-6 p-8 text-center">
          <div className="space-y-2">
            <h1 className="text-2xl font-semibold tracking-tight">Something went wrong</h1>
            <p className="mx-auto max-w-md text-sm text-muted-foreground">
              An unexpected error occurred. Please try again or return to the home page.
            </p>
          </div>
          <div className="flex gap-3">
            <Button onClick={reset} variant="outline">
              Try again
            </Button>
            <Button asChild variant="brand">
              <a href="/">Go to home</a>
            </Button>
          </div>
        </main>
      </body>
    </html>
  );
}

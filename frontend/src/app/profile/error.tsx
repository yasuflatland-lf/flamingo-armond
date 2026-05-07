"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";

type ErrorPageProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function ProfileError({ error, reset }: ErrorPageProps) {
  useEffect(() => {
    // UNAUTHENTICATED errors are intercepted in page.tsx (RSC) and redirect
    // to /login before this boundary is reached. Errors arriving here are
    // non-auth failures (network, 5xx, unexpected GraphQL errors).
    console.error("[/profile error boundary]", {
      message: error.message,
      digest: error.digest,
    });
  }, [error]);

  return (
    <main className="p-8">
      <h1 className="mb-3 text-2xl font-semibold">Couldn&apos;t load your profile</h1>
      <p className="mb-6 text-sm text-muted-foreground">
        Something went wrong while loading your profile. Please try again in a moment.
      </p>
      <Button onClick={reset} variant="outline">
        Retry
      </Button>
    </main>
  );
}

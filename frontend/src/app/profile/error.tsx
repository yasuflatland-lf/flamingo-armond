"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";

type ErrorPageProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function ProfileError({ error, reset }: ErrorPageProps) {
  useEffect(() => {
    // Initial-load UNAUTHENTICATED is intercepted in page.tsx and redirects
    // to /login before this boundary is reached. This boundary handles the
    // residual failure modes (network, 5xx, GraphQL errors raised after
    // hydration — including UNAUTHENTICATED from client-side mutations).
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

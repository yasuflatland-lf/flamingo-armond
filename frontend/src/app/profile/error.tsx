"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { Button } from "@/components/ui/button";

type ErrorPageProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function ProfileError({ error, reset }: ErrorPageProps) {
  const router = useRouter();

  useEffect(() => {
    // Redirect to /login when the underlying GraphQL error is UNAUTHENTICATED.
    // gqlFetch stringifies errors as `GraphQL errors: [...{ "extensions": { "code": "UNAUTHENTICATED" } }...]`.
    if (error.message.includes("UNAUTHENTICATED")) {
      router.replace("/login");
    }
  }, [error, router]);

  return (
    <main className="mx-auto max-w-xl p-8">
      <h1 className="mb-3 text-2xl font-semibold">{"Couldn't load your profile"}</h1>
      <p className="mb-6 text-sm text-muted-foreground">
        Something went wrong while loading your profile. Please try again in a moment.
      </p>
      <Button onClick={reset} variant="outline">
        Retry
      </Button>
    </main>
  );
}

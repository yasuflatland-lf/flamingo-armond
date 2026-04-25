"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";

type ErrorPageProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function ProfileError({ error, reset }: ErrorPageProps) {
  const router = useRouter();
  const redirected = useRef(false);

  useEffect(() => {
    // Always log so observability sees every error reaching this boundary,
    // including the Next.js digest used to correlate with server logs.
    console.error("[/profile error boundary]", {
      message: error.message,
      digest: error.digest,
    });

    // Substring detection: gqlFetch stringifies GraphQL errors via JSON,
    // so a backend `extensions.code = "UNAUTHENTICATED"` ends up literal in
    // the message. Network/HTTP failures fall through to the generic UI.
    if (!redirected.current && error.message.includes("UNAUTHENTICATED")) {
      redirected.current = true;
      router.replace("/login");
    }
  }, [error, router]);

  return (
    <main className="mx-auto max-w-xl p-8">
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

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
    console.error("[/profile error boundary]", {
      message: error.message,
      digest: error.digest,
    });

    // Substring match: gqlFetch JSON-stringifies GraphQL errors, so the
    // backend `extensions.code = "UNAUTHENTICATED"` appears literally in
    // the message. Network/HTTP failures fall through to the generic UI.
    if (!redirected.current && error.message.includes("UNAUTHENTICATED")) {
      redirected.current = true;
      router.replace("/login");
    }
  }, [error, router]);

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

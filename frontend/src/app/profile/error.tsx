"use client";

import { useTranslations } from "next-intl";
import { useEffect } from "react";
import { Button } from "@/components/ui/button";

type ErrorPageProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function ProfileError({ error, reset }: ErrorPageProps) {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");

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
      <h1 className="mb-3 text-2xl font-semibold">{t("couldntLoad")}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t("loadError")}</p>
      <Button onClick={reset} variant="outline">
        {tCommon("retry")}
      </Button>
    </main>
  );
}

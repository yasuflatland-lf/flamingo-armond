"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";

export function CardFetchMoreError({ message, onRetry }: { message: string; onRetry: () => void }) {
  const tCommon = useTranslations("Common");
  return (
    <ErrorBanner
      className="mt-3 flex flex-col items-center gap-2"
      data-testid="cards-fetch-more-error"
    >
      <span>{message}</span>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        {tCommon("retry")}
      </Button>
    </ErrorBanner>
  );
}

"use client";

import { useTranslations } from "next-intl";
import { useIsOffline } from "@/lib/network/use-network-status";

export function OfflineBanner() {
  const t = useTranslations("Pwa");
  const isOffline = useIsOffline();

  if (!isOffline) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      data-testid="offline-banner"
      className="bg-warning px-4 py-3 text-warning-foreground"
    >
      <p className="font-semibold text-sm">{t("offlineHeading")}</p>
      <p className="text-sm">{t("offlineBody")}</p>
    </div>
  );
}

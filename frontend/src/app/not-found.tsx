"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  const t = useTranslations("NotFound");

  return (
    <main className="flex min-h-[calc(100dvh-3rem)] w-full flex-col items-center justify-center gap-6 p-8 text-center">
      <div className="space-y-2">
        <p className="text-8xl font-bold tracking-tight text-brand-primary sm:text-9xl">404</p>
        <h1 className="text-2xl font-semibold tracking-tight">{t("heading")}</h1>
        <p className="mx-auto max-w-md text-sm text-muted-foreground">{t("message")}</p>
      </div>

      <Button asChild variant="brand">
        <Link href="/">Back to Home</Link>
      </Button>
    </main>
  );
}

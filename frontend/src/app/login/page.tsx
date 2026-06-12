import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import type { ReactNode } from "react";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { InAppBrowserNotice } from "./in-app-browser-notice";
import { LoginButton } from "./login-button";

export const metadata: Metadata = { title: "Login" };

type SearchParams = Promise<{ error?: string }>;

export default async function LoginPage({ searchParams }: { searchParams: SearchParams }) {
  // Redirect an already-authenticated visitor away. The middleware forwards
  // identity via x-auth-status; stale / anonymous / error all render the page.
  if (readAuthContext(await headers()).status === "authenticated") redirect("/cardgroups");

  const t = await getTranslations("Login");
  const { error } = await searchParams;

  // Both footer rich-text tags render the same styled link, differing only in href.
  const footerLink = (href: string) => (chunks: ReactNode) => (
    <a href={href} className="underline underline-offset-2 hover:text-foreground">
      {chunks}
    </a>
  );

  return (
    <main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
      <div className="flex flex-col">
        <div
          data-testid="form-brand-header"
          className="lg:hidden flex items-center justify-center gap-2 pt-8"
        >
          <FlamingoMark className="size-7" />
          <span className="text-base font-medium">flamingo-armond</span>
        </div>

        <div className="flex flex-1 items-center justify-center p-6 sm:p-8">
          <div className="w-full max-w-sm">
            <div className="flex flex-col gap-6 rounded-2xl border bg-card p-8 shadow-[0_8px_40px_-12px_rgba(0,0,0,0.18)]">
              <div className="flex flex-col gap-2 text-start">
                <h1 className="text-2xl font-semibold tracking-tight">{t("heading")}</h1>
                <p className="text-sm text-muted-foreground">{t("googleCta")}</p>
              </div>

              {error && (
                <p role="alert" className="text-sm text-destructive">
                  {t("signInFailed", { error })}
                </p>
              )}

              <InAppBrowserNotice />

              <LoginButton />

              <footer className="border-t pt-5 text-xs text-muted-foreground text-center">
                {t.rich("terms", {
                  terms: footerLink("/terms"),
                  privacy: footerLink("/privacy"),
                })}
              </footer>
            </div>
          </div>
        </div>
      </div>

      <div
        data-testid="brand-panel"
        className="max-lg:hidden relative flex items-center justify-center overflow-hidden bg-gradient-to-br from-[oklch(83%_0.11_22)] via-[oklch(72%_0.185_18.45)] to-[oklch(60%_0.2_13)]"
      >
        {/* Soft radial glows give the flat coral gradient depth and atmosphere. */}
        <div className="pointer-events-none absolute -top-24 -left-16 size-96 rounded-full bg-white/20 blur-3xl" />
        <div className="pointer-events-none absolute -right-12 -bottom-32 size-[30rem] rounded-full bg-[rgba(150,25,60,0.45)] blur-3xl" />

        {/* Horizontal brand lockup: white-framed logo beside the wordmark. The
            mark is itself coral, so a white badge lifts it off the pink ground. */}
        <div className="relative flex items-center gap-5">
          <div className="rounded-[1.3rem] bg-white p-2.5 shadow-xl shadow-rose-950/20">
            <FlamingoMark className="size-16" />
          </div>
          <div className="flex flex-col gap-1.5">
            <span className="text-4xl font-semibold tracking-tight text-white drop-shadow-[0_2px_10px_rgba(120,20,40,0.45)]">
              flamingo-armond
            </span>
            <span className="text-sm font-medium text-white/90 drop-shadow-[0_1px_6px_rgba(120,20,40,0.35)]">
              {t("tagline")}
            </span>
          </div>
        </div>
      </div>
    </main>
  );
}

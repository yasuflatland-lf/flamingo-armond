import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";
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

  // Locale-aware tracking, mirroring apple.com/jp's locale-specific
  // letter-spacing: Latin display text takes tight negative tracking, while
  // Japanese takes near-zero / slightly positive tracking (negative tracking
  // cramps kana) plus the wider leading dense Japanese glyphs need. Proportional
  // metrics (palt) are enabled globally for `:lang(ja)` in globals.css.
  const isJa = (await getLocale()) === "ja";

  // Both footer rich-text tags render the same styled link, differing only in href.
  const footerLink = (href: string) => (chunks: ReactNode) => (
    <a href={href} className="underline underline-offset-2 hover:text-foreground">
      {chunks}
    </a>
  );

  return (
    // Even split: brand panel (50%) left, form column (50%) right. The
    // `grid-cols-2` template divides the viewport equally so the panel boundary
    // sits at the centre; below lg the brand panel is hidden and the form takes
    // the full width.
    <main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
      {/* LEFT — brand panel (lg+ only). First grid child so it occupies the left
          half. */}
      <div
        data-testid="brand-panel"
        className="max-lg:hidden relative flex items-center justify-center overflow-hidden bg-gradient-to-br from-[oklch(83%_0.11_22)] via-[oklch(72%_0.185_18.45)] to-[oklch(60%_0.2_13)]"
      >
        {/* Soft radial glows give the flat coral gradient depth and atmosphere. */}
        <div className="pointer-events-none absolute -top-24 -left-16 size-96 rounded-full bg-white/20 blur-3xl" />
        <div className="pointer-events-none absolute -right-12 -bottom-32 size-[30rem] rounded-full bg-[rgba(150,25,60,0.45)] blur-3xl" />
        {/* Right-edge vignette: a hint of depth where the panel meets the form. */}
        <div className="pointer-events-none absolute inset-y-0 right-0 w-32 bg-gradient-to-l from-[rgba(120,20,40,0.16)] to-transparent" />

        {/* Centered vertical lockup: bare app-mark over wordmark + tagline. Sizes
            follow a ~2.5x modular scale — tagline 16 → wordmark 40 → mark 104 — so
            the hierarchy steps cleanly without the wordmark shouting. Scaled down
            from the earlier oversized lockup and tightened to mirror apple.com/jp's
            restraint: a small 8px gap binds the wordmark to its tagline, a larger
            24px gap sets that text block apart from the mark above. */}
        <div className="relative flex flex-col items-center gap-[24px] px-[48px] text-center">
          <FlamingoMark background={false} className="size-[104px]" />
          <div className="flex flex-col items-center gap-[8px]">
            {/* Wordmark is always Latin → tight Apple-display negative tracking. */}
            <span className="text-[40px] font-semibold leading-[1.1] tracking-[-0.02em] text-white drop-shadow-[0_1px_8px_rgba(120,20,40,0.4)]">
              Flamingo Armond
            </span>
            <span
              className={`text-[16px] font-medium text-white/90 drop-shadow-[0_1px_4px_rgba(120,20,40,0.3)] ${
                isJa ? "leading-[1.6] tracking-[0.03em]" : "leading-[1.45] tracking-[-0.01em]"
              }`}
            >
              {t("tagline")}
            </span>
          </div>
        </div>
      </div>

      {/* RIGHT — form column. Second grid child → right half at lg+,
          full-width and centered below lg. */}
      <div className="flex flex-col">
        <div
          data-testid="form-brand-header"
          className="lg:hidden flex items-center justify-center gap-[13px] pt-[34px]"
        >
          <FlamingoMark className="size-7" />
          <span className="text-base font-medium tracking-[-0.01em]">Flamingo Armond</span>
        </div>

        <div className="flex flex-1 items-center justify-center px-6 py-[34px] sm:px-8 lg:px-12 xl:px-16">
          {/* 377px is a Fibonacci number — a deliberately calm form measure. */}
          <div className="w-full max-w-[377px]">
            <div className="flex flex-col gap-[21px] rounded-[21px] border border-border/70 bg-card p-[34px] shadow-[0_1px_2px_rgba(0,0,0,0.04),0_12px_32px_-12px_rgba(0,0,0,0.12)]">
              <div className="flex flex-col gap-[13px] text-start">
                <h1
                  className={`text-[28px] font-semibold ${
                    isJa ? "leading-[1.25] tracking-[0.01em]" : "leading-[1.15] tracking-[-0.018em]"
                  }`}
                >
                  {t("heading")}
                </h1>
                <p
                  className={`text-[15px] text-muted-foreground ${
                    isJa ? "leading-[1.8] tracking-[0.02em]" : "leading-[1.55] tracking-[-0.006em]"
                  }`}
                >
                  {t("googleCta")}
                </p>
              </div>

              {error && (
                <p role="alert" className="text-sm text-destructive">
                  {t("signInFailed", { error })}
                </p>
              )}

              <InAppBrowserNotice />

              <LoginButton />

              <footer
                className={`border-t pt-[21px] text-[12px] text-muted-foreground text-center ${
                  isJa ? "leading-[1.7] tracking-[0.01em]" : "leading-[1.5]"
                }`}
              >
                {t.rich("terms", {
                  terms: footerLink("/terms"),
                  privacy: footerLink("/privacy"),
                })}
              </footer>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}

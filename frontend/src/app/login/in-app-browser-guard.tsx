"use client";

import { useTranslations } from "next-intl";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { isAppSpecificInAppBrowser } from "@/lib/in-app-browser";
import { type InAppBrowserEscape, useInAppBrowserEscape } from "./use-in-app-browser-escape";

/**
 * Steers visitors out of an embedded in-app browser (where Google blocks OAuth
 * with `403 disallowed_useragent`) and into the system browser. App-specific
 * detections (LINE, Instagram, …) get an unmissable blocking modal; the generic
 * Android WebView token — the highest false-positive vector — keeps a low-key
 * inline banner so a misdetection never hard-locks a legitimate visitor.
 */
export function InAppBrowserGuard() {
  const ctx = useInAppBrowserEscape();

  if (!ctx.browser) return null;
  if (isAppSpecificInAppBrowser(ctx.browser)) return <InAppBrowserModal ctx={ctx} />;
  return <InAppBrowserBanner ctx={ctx} />;
}

function InAppBrowserModal({ ctx }: { ctx: InAppBrowserEscape }) {
  const t = useTranslations("Login");
  const { browser, copied, openInExternalBrowser, copyPageLink } = ctx;

  return (
    // `open` is pinned true and there is no cancel/close affordance: inside an
    // in-app browser OAuth fails regardless, so the external browser is the only
    // path forward. Escape is disabled; Radix AlertDialog already ignores
    // outside-click and renders no close button.
    <AlertDialog open>
      <AlertDialogContent onEscapeKeyDown={(e) => e.preventDefault()}>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("inAppBrowserModalTitle")}</AlertDialogTitle>
          <AlertDialogDescription>{t("inAppBrowserWarning")}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          {browser === "line" ? (
            <Button type="button" variant="brand" onClick={openInExternalBrowser}>
              {t("openInBrowser")}
            </Button>
          ) : (
            <Button type="button" variant="brand" onClick={copyPageLink}>
              {copied ? t("linkCopied") : t("copyLink")}
            </Button>
          )}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function InAppBrowserBanner({ ctx }: { ctx: InAppBrowserEscape }) {
  const t = useTranslations("Login");
  const { copied, copyPageLink } = ctx;

  return (
    <div
      role="alert"
      className="flex flex-col gap-[13px] rounded-[13px] border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900"
    >
      <p className="font-medium leading-snug">{t("inAppBrowserWarning")}</p>
      <Button type="button" variant="outline" size="sm" onClick={copyPageLink}>
        {copied ? t("linkCopied") : t("copyLink")}
      </Button>
    </div>
  );
}

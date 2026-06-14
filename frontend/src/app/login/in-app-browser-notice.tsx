"use client";

import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { detectInAppBrowser, type InAppBrowser } from "@/lib/in-app-browser";

// LINE honours this query param by reopening the URL in the OS default browser.
// https://developers.line.biz/en/docs/line-login/using-line-url-scheme/
const LINE_EXTERNAL_BROWSER_PARAM = "openExternalBrowser";

/**
 * Advisory banner shown only when the visitor is inside an in-app browser
 * (LINE, Instagram, Facebook, …), where Google blocks OAuth sign-in with
 * `Error 403: disallowed_useragent`. It guides the user into the system
 * browser instead of letting them hit the opaque Google error after tapping
 * the sign-in button. The sign-in button itself stays enabled so a false
 * positive never locks anyone out.
 */
export function InAppBrowserNotice() {
  const t = useTranslations("Login");
  // Start hidden: the server cannot read the user-agent, so detection runs only
  // after mount. Rendering nothing on the server also avoids a hydration mismatch.
  const [browser, setBrowser] = useState<InAppBrowser | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setBrowser(detectInAppBrowser(navigator.userAgent));
  }, []);

  if (!browser) return null;

  function openInExternalBrowser() {
    // LINE re-opens the page in the external browser when the flag is present.
    const url = new URL(window.location.href);
    url.searchParams.set(LINE_EXTERNAL_BROWSER_PARAM, "1");
    window.location.href = url.toString();
  }

  async function copyPageLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied(true);
    } catch {
      // Clipboard API may be unavailable (insecure context / denied permission).
      // The banner text already tells the user how to proceed manually.
      setCopied(false);
    }
  }

  return (
    <div
      role="alert"
      className="flex flex-col gap-[13px] rounded-[13px] border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900"
    >
      <p className="font-medium leading-snug">{t("inAppBrowserWarning")}</p>
      {browser === "line" ? (
        <Button type="button" variant="outline" size="sm" onClick={openInExternalBrowser}>
          {t("openInBrowser")}
        </Button>
      ) : (
        <Button type="button" variant="outline" size="sm" onClick={copyPageLink}>
          {copied ? t("linkCopied") : t("copyLink")}
        </Button>
      )}
    </div>
  );
}

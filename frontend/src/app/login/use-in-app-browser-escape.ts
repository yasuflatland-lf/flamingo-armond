"use client";

import { useEffect, useState } from "react";
import { detectInAppBrowser, type InAppBrowser } from "@/lib/in-app-browser";

// LINE honours this query param by reopening the URL in the OS default browser.
// https://developers.line.biz/en/docs/line-login/using-line-url-scheme/
const LINE_EXTERNAL_BROWSER_PARAM = "openExternalBrowser";

export type InAppBrowserEscape = {
  /** Detected in-app browser, or null in a standalone browser / before mount. */
  browser: InAppBrowser | null;
  /** True after a successful copyPageLink(). */
  copied: boolean;
  /** LINE-only: reopen the current URL in the OS default browser. */
  openInExternalBrowser: () => void;
  /** Copy the current URL so the user can paste it into Safari / Chrome. */
  copyPageLink: () => Promise<void>;
};

/**
 * Client-only detection of an embedded in-app browser plus the two escape
 * actions the login page offers. Detection runs in a mount effect because the
 * server cannot read the user-agent; starting null also avoids a hydration
 * mismatch.
 */
export function useInAppBrowserEscape(): InAppBrowserEscape {
  const [browser, setBrowser] = useState<InAppBrowser | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setBrowser(detectInAppBrowser(navigator.userAgent));
  }, []);

  function openInExternalBrowser() {
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
      // The confirmation text already tells the user how to proceed manually.
      setCopied(false);
    }
  }

  return { browser, copied, openInExternalBrowser, copyPageLink };
}

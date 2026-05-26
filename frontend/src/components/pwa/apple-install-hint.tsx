"use client";

import { useEffect, useState } from "react";

// localStorage key used to remember that the user dismissed the banner.
const DISMISSED_KEY = "pwa-ios-install-hint-dismissed";

// Returns true when all three conditions for showing the hint are satisfied:
//   (a) The UA string indicates an iOS device (iPhone/iPad/iPod) and is not
//       the IE/Edge MSStream quirk that also sets window.MSStream.
//   (b) The app is NOT already running in standalone / installed mode.
//   (c) The user has not previously dismissed the hint.
// Every browser API access is wrapped in try/catch so a missing API (e.g.
// localStorage in a restricted environment) never throws.
function shouldShowHint(): boolean {
  try {
    const ua = navigator.userAgent;
    const isIOS = /iphone|ipad|ipod/i.test(ua) && !(window as { MSStream?: unknown }).MSStream;
    if (!isIOS) return false;

    const isStandaloneMedia = window.matchMedia("(display-mode: standalone)").matches;
    const isStandaloneNavigator = (navigator as { standalone?: boolean }).standalone === true;
    if (isStandaloneMedia || isStandaloneNavigator) return false;

    const dismissed = localStorage.getItem(DISMISSED_KEY) === "true";
    if (dismissed) return false;

    return true;
  } catch {
    return false;
  }
}

export function AppleInstallHint() {
  // Start hidden to avoid a hydration mismatch — the server never knows
  // whether the client is an iOS device or has already dismissed the hint.
  const [visible, setVisible] = useState<boolean>(false);

  useEffect(() => {
    setVisible(shouldShowHint());
  }, []);

  if (!visible) return null;

  function handleDismiss() {
    try {
      localStorage.setItem(DISMISSED_KEY, "true");
    } catch {
      // localStorage may be unavailable in private/restricted contexts — ignore.
    }
    setVisible(false);
  }

  return (
    <section
      aria-label="Install hint"
      className="fixed inset-x-0 bottom-0 z-50 mx-auto max-w-md rounded-t-2xl bg-brand-primary px-4 pb-[env(safe-area-inset-bottom,1rem)] pt-4 shadow-lg"
    >
      <div className="flex items-start justify-between gap-3 pb-4">
        <div className="flex flex-col gap-1">
          <p className="font-semibold text-brand-primary-foreground text-sm leading-snug">
            Install flamingo
          </p>
          <p className="text-brand-primary-foreground text-xs opacity-90">
            Tap the Share button, then &ldquo;Add to Home Screen&rdquo;.
          </p>
        </div>
        <button
          type="button"
          aria-label="Dismiss install hint"
          onClick={handleDismiss}
          className="mt-0.5 flex-shrink-0 rounded-full p-1 text-brand-primary-foreground opacity-80 hover:opacity-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-primary-foreground"
        >
          {/* X icon drawn inline — no icon-library dependency needed */}
          <svg
            aria-hidden="true"
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M12 4L4 12M4 4l8 8"
              stroke="currentColor"
              strokeWidth="1.75"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </div>
    </section>
  );
}

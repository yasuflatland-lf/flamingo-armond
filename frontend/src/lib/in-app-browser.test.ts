import { describe, expect, it } from "vitest";
import { detectInAppBrowser, isAppSpecificInAppBrowser } from "./in-app-browser";

// Representative user-agent strings captured from real devices. The exact
// version numbers are irrelevant — only the app-identifying tokens matter.
const IN_APP: ReadonlyArray<readonly [string, ReturnType<typeof detectInAppBrowser>]> = [
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Safari/604.1 Line/14.5.0",
    "line",
  ],
  [
    "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.0.0 Mobile Safari/537.36 Line/14.5.0/IAB",
    "line",
  ],
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/468.0.0;FBBV/1]",
    "facebook",
  ],
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 333.0.0.0",
    "instagram",
  ],
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Twitter for iPhone",
    "twitter",
  ],
  [
    "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.0.0 Mobile Safari/537.36 musical_ly_2023 TikTok",
    "tiktok",
  ],
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.44",
    "wechat",
  ],
  [
    "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.0.0 Mobile Safari/537.36 KAKAOTALK 10.4.5",
    "kakaotalk",
  ],
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Snapchat/12.0",
    "snapchat",
  ],
  [
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [Pinterest/iOS]",
    "pinterest",
  ],
  [
    // Generic Android System WebView (no app-specific token) — the `; wv)` marker.
    "Mozilla/5.0 (Linux; Android 14; Pixel 8 Build/UP1A; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.0.0 Mobile Safari/537.36",
    "webview",
  ],
];

// Standalone browsers must NOT be flagged — a false positive would nag a user
// for whom Google sign-in works fine.
const STANDALONE: readonly string[] = [
  // Mobile Safari (iOS) — the canonical browser that DOES allow OAuth.
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
  // Chrome on Android (no `wv` token).
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36",
  // Desktop Chrome.
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
  // Desktop Firefox.
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:127.0) Gecko/20100101 Firefox/127.0",
];

describe("detectInAppBrowser", () => {
  it.each(IN_APP)("flags %s as %s", (ua, expected) => {
    expect(detectInAppBrowser(ua)).toBe(expected);
  });

  it.each(STANDALONE)("returns null for standalone browser %s", (ua) => {
    expect(detectInAppBrowser(ua)).toBeNull();
  });

  it("returns null for an empty user-agent", () => {
    expect(detectInAppBrowser("")).toBeNull();
  });
});

describe("isAppSpecificInAppBrowser", () => {
  it("returns true for app-specific signatures", () => {
    expect(isAppSpecificInAppBrowser("line")).toBe(true);
    expect(isAppSpecificInAppBrowser("instagram")).toBe(true);
    expect(isAppSpecificInAppBrowser("facebook")).toBe(true);
  });

  it("returns false for the generic Android WebView token", () => {
    expect(isAppSpecificInAppBrowser("webview")).toBe(false);
  });
});

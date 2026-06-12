/**
 * Best-effort detection of embedded in-app browsers (WebViews) shipped inside
 * native apps such as LINE, Instagram, or Facebook.
 *
 * Google blocks OAuth 2.0 sign-in from embedded user-agents with
 * `Error 403: disallowed_useragent` — see Google's "Use secure browsers"
 * policy (https://developers.google.com/identity/protocols/oauth2/policies).
 * The platform deliberately refuses these requests, so the only viable remedy
 * is to detect the in-app browser and steer the user into the system browser
 * (Safari / Chrome), which the login page does.
 *
 * Detection is intentionally NOT used to hard-block the sign-in button: a false
 * positive only adds an advisory banner rather than locking a legitimate
 * visitor out.
 */
export type InAppBrowser =
  | "line"
  | "facebook"
  | "instagram"
  | "twitter"
  | "tiktok"
  | "wechat"
  | "kakaotalk"
  | "snapchat"
  | "pinterest"
  | "webview";

// Ordered so app-specific markers win over the generic Android WebView token:
// an Instagram WebView UA also carries `; wv)`, but "instagram" is the useful
// label and is what unlocks any app-specific escape hatch.
const SIGNATURES: ReadonlyArray<readonly [InAppBrowser, RegExp]> = [
  ["line", /\bLine\//i],
  ["facebook", /\bFB(?:AN|AV|_IAB|IOS|SS)\b/],
  ["instagram", /\bInstagram\b/i],
  ["twitter", /\bTwitter(?:Android)?\b/i],
  ["tiktok", /\b(?:musical_ly|BytedanceWebview|TikTok|Trill)\b/i],
  ["wechat", /\bMicroMessenger\b/i],
  ["kakaotalk", /\bKAKAOTALK\b/i],
  ["snapchat", /\bSnapchat\b/i],
  ["pinterest", /\bPinterest\b/i],
  // Standard Android System WebView marker. Real Chrome / Custom Tabs omit it.
  ["webview", /;\s?wv[);]/],
];

/**
 * Returns the recognised in-app browser for a user-agent string, or `null` when
 * the UA looks like a normal standalone browser. The first matching signature
 * wins, so app-specific labels take precedence over the generic WebView token.
 */
export function detectInAppBrowser(userAgent: string): InAppBrowser | null {
  for (const [kind, pattern] of SIGNATURES) {
    if (pattern.test(userAgent)) return kind;
  }
  return null;
}

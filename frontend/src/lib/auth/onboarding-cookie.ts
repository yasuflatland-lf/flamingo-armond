/**
 * HMAC-signed "this account finished onboarding" hint cookie. It exists so the
 * middleware gate can answer the display-name question without a backend round
 * trip on every navigation. The MAC covers the caller's `sub`, so the value is
 * neither forgeable by the user nor transferable to another account.
 */

/** Cookie name. `fa-` prefix matches nothing Supabase writes (`sb-...`). */
export const ONBOARDING_COOKIE_NAME = "fa-onboarded";

/**
 * Bounded staleness window. A display name cleared out-of-band (an admin edit)
 * re-gates within this long at worst, at a cost of one backend lookup per user
 * per window.
 */
export const ONBOARDING_COOKIE_TTL_SECONDS = 60 * 60;

const FORMAT_VERSION = "v1";

const keyCache = new Map<string, Promise<CryptoKey>>();

function hmacKey(secret: string): Promise<CryptoKey> {
  const cached = keyCache.get(secret);
  if (cached !== undefined) return cached;
  const key = crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  keyCache.set(secret, key);
  return key;
}

async function computeMac(secret: string, sub: string, expSeconds: number): Promise<string> {
  const key = await hmacKey(secret);
  const payload = `${FORMAT_VERSION}:${sub}:${expSeconds}`;
  const signature = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(payload));
  return Buffer.from(signature).toString("base64url");
}

/**
 * Length-checked, branch-free string compare. The MAC is recomputed server-side
 * on every verify, so an early-exit compare would leak it one character at a
 * time to a caller who can time the middleware.
 */
function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) {
    diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return diff === 0;
}

/** Mints a cookie value asserting that `sub` is onboarded, valid for the TTL. */
export async function signOnboardingCookie(
  secret: string,
  sub: string,
  nowMs: number = Date.now(),
): Promise<string> {
  const exp = Math.floor(nowMs / 1000) + ONBOARDING_COOKIE_TTL_SECONDS;
  return `${FORMAT_VERSION}.${exp}.${await computeMac(secret, sub, exp)}`;
}

/**
 * True only for a well-formed, unexpired value whose MAC was minted for this
 * exact `sub` under this exact secret. Every other input — absent, malformed,
 * wrong version, expired, another account's cookie — is false, which sends the
 * gate down the authoritative backend-lookup path.
 */
export async function verifyOnboardingCookie(
  value: string | undefined,
  secret: string,
  sub: string,
  nowMs: number = Date.now(),
): Promise<boolean> {
  if (value === undefined || value === "" || sub === "") return false;

  const parts = value.split(".");
  if (parts.length !== 3) return false;
  const [version, expRaw, mac] = parts;
  if (version !== FORMAT_VERSION || expRaw === undefined || mac === undefined) return false;
  if (!/^\d+$/.test(expRaw)) return false;

  const exp = Number(expRaw);
  const nowSeconds = Math.floor(nowMs / 1000);
  // Upper bound as well as lower: a cookie minted under a longer TTL must stop
  // being honoured the moment the TTL is shortened, not one old window later.
  if (exp <= nowSeconds || exp > nowSeconds + ONBOARDING_COOKIE_TTL_SECONDS) return false;

  return timingSafeEqual(mac, await computeMac(secret, sub, exp));
}

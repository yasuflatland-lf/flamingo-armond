import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { buildSchema, parse, validate } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { signOnboardingCookie } from "./onboarding-cookie";
import {
  isOnboardingGateExempt,
  ONBOARDING_ENTRY_PATH,
  ONBOARDING_GATE_QUERY,
  resolveOnboardingGate,
} from "./onboarding-gate";

const SECRET = "test-secret-that-is-at-least-32-characters";
const SUB = "11111111-1111-4111-8111-111111111111";
const BACKEND_URL = "http://backend.test";
const NOW = 1_700_000_000_000;

/** Every route shape named in the acceptance criteria, one representative each. */
const PROTECTED_PATHS = [
  "/",
  "/learn/33333333-3333-4333-8333-333333333333",
  "/cardgroups",
  "/cardgroups/new",
  "/catalog",
  "/stats",
  "/profile",
  "/admin/users",
];

function meResponse(displayName: string | null) {
  return {
    ok: true,
    json: async () => ({ data: { me: { id: SUB, displayName } } }),
  } as unknown as Response;
}

function gateInput(overrides: Partial<Parameters<typeof resolveOnboardingGate>[0]> = {}) {
  return {
    pathname: "/cardgroups",
    sub: SUB,
    cookieValue: undefined,
    secret: SECRET,
    backendUrl: BACKEND_URL,
    getAccessToken: async () => "access-token",
    nowMs: NOW,
    ...overrides,
  };
}

describe("isOnboardingGateExempt", () => {
  it.each([
    "/onboarding",
    "/onboarding/start",
    "/login",
    "/auth/callback",
    "/api/healthz",
    "/api/ping",
    "/_next/static/chunk.js",
    "/terms",
    "/privacy",
    "/favicon.ico",
    "/sw.js",
    "/offline.html",
    "/manifest.webmanifest",
  ])("exempts %s so the gate cannot loop or break a probe", (pathname) => {
    expect(isOnboardingGateExempt(pathname)).toBe(true);
  });

  it.each(PROTECTED_PATHS)("does not exempt %s", (pathname) => {
    expect(isOnboardingGateExempt(pathname)).toBe(false);
  });

  it("matches on a path boundary, not a bare prefix", () => {
    expect(isOnboardingGateExempt("/loginate")).toBe(false);
    expect(isOnboardingGateExempt("/onboardingx")).toBe(false);
  });
});

describe("ONBOARDING_GATE_QUERY", () => {
  it("is a valid operation against the shared schema", () => {
    const schemaDir = resolve(dirname(fileURLToPath(import.meta.url)), "../../../../schema");
    const sdl = readdirSync(schemaDir)
      .filter((file) => file.endsWith(".graphql"))
      .sort()
      .map((file) => readFileSync(join(schemaDir, file), "utf8"))
      .join("\n");
    const errors = validate(buildSchema(sdl), parse(ONBOARDING_GATE_QUERY));
    expect(errors).toEqual([]);
  });
});

describe("resolveOnboardingGate", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    fetchMock.mockReset();
  });

  it.each(
    PROTECTED_PATHS,
  )("redirects a signed-in user with an empty display name away from %s", async (pathname) => {
    fetchMock.mockResolvedValue(meResponse(""));
    const result = await resolveOnboardingGate(gateInput({ pathname }));
    expect(result.redirectTo).toBe(ONBOARDING_ENTRY_PATH);
    expect(result.clearCookie).toBe(true);
    expect(result.setCookie).toBeNull();
  });

  it.each([
    "/onboarding",
    "/onboarding/start",
    "/login",
  ])("leaves the same user free to reach %s", async (pathname) => {
    const result = await resolveOnboardingGate(gateInput({ pathname }));
    expect(result.redirectTo).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it.each(PROTECTED_PATHS)("does not disturb an onboarded user on %s", async (pathname) => {
    fetchMock.mockResolvedValue(meResponse("Alice"));
    const result = await resolveOnboardingGate(gateInput({ pathname }));
    expect(result.redirectTo).toBeNull();
    expect(result.clearCookie).toBe(false);
    expect(result.setCookie).not.toBeNull();
  });

  it("treats a whitespace-only display name as not onboarded (shares isUserOnboarded)", async () => {
    fetchMock.mockResolvedValue(meResponse("   "));
    const result = await resolveOnboardingGate(gateInput());
    expect(result.redirectTo).toBe(ONBOARDING_ENTRY_PATH);
  });

  it("issues a cookie the next request can verify, skipping the lookup", async () => {
    fetchMock.mockResolvedValue(meResponse("Alice"));
    const first = await resolveOnboardingGate(gateInput());
    expect(fetchMock).toHaveBeenCalledTimes(1);

    fetchMock.mockClear();
    const second = await resolveOnboardingGate(gateInput({ cookieValue: first.setCookie ?? "" }));
    expect(second.redirectTo).toBeNull();
    expect(second.setCookie).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("ignores a cookie minted for a different sub and re-checks the backend", async () => {
    const foreign = await signOnboardingCookie(SECRET, "99999999-9999-4999-8999-999999999999", NOW);
    fetchMock.mockResolvedValue(meResponse(""));
    const result = await resolveOnboardingGate(gateInput({ cookieValue: foreign }));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(result.redirectTo).toBe(ONBOARDING_ENTRY_PATH);
  });

  it("sends the caller's bearer token and the gate query to the backend", async () => {
    fetchMock.mockResolvedValue(meResponse("Alice"));
    await resolveOnboardingGate(gateInput());
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`${BACKEND_URL}/query`);
    const headers = (init?.headers ?? {}) as Record<string, string>;
    expect(headers.authorization).toBe("Bearer access-token");
    expect(String(init?.body)).toContain("OnboardingGateMe");
  });

  it("looks up on every request when no secret is configured", async () => {
    fetchMock.mockResolvedValue(meResponse("Alice"));
    const result = await resolveOnboardingGate(gateInput({ secret: undefined }));
    expect(result.redirectTo).toBeNull();
    expect(result.setCookie).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("still redirects an un-onboarded user when no secret is configured", async () => {
    fetchMock.mockResolvedValue(meResponse(null));
    const result = await resolveOnboardingGate(gateInput({ secret: undefined }));
    expect(result.redirectTo).toBe(ONBOARDING_ENTRY_PATH);
  });

  it("fails open when the backend is unreachable", async () => {
    fetchMock.mockRejectedValue(new Error("ECONNREFUSED"));
    const result = await resolveOnboardingGate(gateInput());
    expect(result.redirectTo).toBeNull();
    expect(result.setCookie).toBeNull();
  });

  it("fails open on a non-2xx backend response", async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503 } as unknown as Response);
    const result = await resolveOnboardingGate(gateInput());
    expect(result.redirectTo).toBeNull();
  });

  it("fails open when the lookup returns GraphQL errors", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ data: null, errors: [{ message: "boom" }] }),
    } as unknown as Response);
    const result = await resolveOnboardingGate(gateInput());
    expect(result.redirectTo).toBeNull();
  });

  it("fails open when no access token is available", async () => {
    const result = await resolveOnboardingGate(gateInput({ getAccessToken: async () => null }));
    expect(result.redirectTo).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("does nothing without a subject claim", async () => {
    const result = await resolveOnboardingGate(gateInput({ sub: "" }));
    expect(result.redirectTo).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

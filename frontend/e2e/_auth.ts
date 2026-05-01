import type { BrowserContext } from "@playwright/test";
import type { CookieOptions } from "@supabase/ssr";
import { createBrowserClient } from "@supabase/ssr";
import { createClient } from "@supabase/supabase-js";

type RoleName = "admin" | "general";

type SeedUserInput = {
  email: string;
  password: string;
  role: RoleName;
  displayName?: string;
};

type SeedCardgroupInput = {
  ownerId: string;
  name: string;
};

type SeedCardInput = {
  cardgroupId: string;
  front: string;
  back: string;
};

type AuthCookie = {
  name: string;
  value: string;
  options: CookieOptions;
};

const supabaseUrl = requireEnv("E2E_SUPABASE_URL");
const anonKey = requireEnv("E2E_SUPABASE_ANON_KEY");
const serviceRoleKey = requireEnv("E2E_SUPABASE_SERVICE_ROLE_KEY");
const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const appHost = new URL(baseURL).hostname;

const adminClient = createClient(supabaseUrl, serviceRoleKey, {
  auth: {
    autoRefreshToken: false,
    persistSession: false,
  },
});

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing ${name}; start Supabase and export local E2E env vars.`);
  return value;
}

function projectRef(): string {
  const { hostname } = new URL(supabaseUrl);
  if (hostname === "127.0.0.1" || hostname === "localhost") {
    return "127";
  }
  return hostname.split(".")[0] ?? hostname;
}

async function findUserByEmail(email: string) {
  let page = 1;
  for (;;) {
    const { data, error } = await adminClient.auth.admin.listUsers({ page, perPage: 100 });
    if (error) throw error;
    const user = data.users.find((candidate) => candidate.email?.toLowerCase() === email);
    if (user) return user;
    if (data.users.length < 100) return null;
    page += 1;
  }
}

export async function seedUser({ email, password, role, displayName }: SeedUserInput) {
  const existing = await findUserByEmail(email);
  const user =
    existing ??
    (
      await adminClient.auth.admin.createUser({
        email,
        password,
        email_confirm: true,
        user_metadata: { display_name: displayName ?? email },
      })
    ).data.user;

  if (!user) throw new Error(`Could not create user ${email}`);

  if (existing) {
    const { error } = await adminClient.auth.admin.updateUserById(user.id, {
      password,
      email_confirm: true,
      user_metadata: { display_name: displayName ?? email },
    });
    if (error) throw error;
  }

  const { error: userError } = await adminClient.from("users").upsert({
    id: user.id,
    display_name: displayName ?? email,
  });
  if (userError) throw userError;

  await ensureRole(role);
  const { data: roleRow, error: roleError } = await adminClient
    .from("roles")
    .select("id")
    .eq("name", role)
    .single();
  if (roleError) throw roleError;

  const { error: userRoleError } = await adminClient.from("user_roles").upsert({
    user_id: user.id,
    role_id: roleRow.id,
  });
  if (userRoleError) throw userRoleError;

  return { id: user.id, email, password, role };
}

export async function seedCardgroup({ ownerId, name }: SeedCardgroupInput) {
  const { data: existing, error: selectError } = await adminClient
    .from("cardgroups")
    .select("id, name")
    .eq("owner_id", ownerId)
    .eq("name", name)
    .maybeSingle();
  if (selectError) throw selectError;
  if (existing) return existing;

  const { data, error } = await adminClient
    .from("cardgroups")
    .insert({ owner_id: ownerId, name })
    .select("id, name")
    .single();
  if (error) throw error;
  return data;
}

export async function seedCards(cards: SeedCardInput[]) {
  if (cards.length === 0) return [];
  const now = new Date().toISOString();
  const rows = cards.map((card) => ({
    cardgroup_id: card.cardgroupId,
    front: card.front,
    back: card.back,
    due: now,
    stability: 2.5,
    difficulty: 5,
    elapsed_days: 0,
    scheduled_days: 0,
    reps: 0,
    lapses: 0,
    state: 0,
    last_review: now,
  }));

  const { data, error } = await adminClient
    .from("cards")
    .upsert(rows, { onConflict: "cardgroup_id,front" })
    .select("id, front, back, cardgroup_id");
  if (error) throw error;
  return data;
}

export async function loginAs(
  context: BrowserContext,
  credentials: { email: string; password: string },
) {
  const cookiesToSet: AuthCookie[] = [];
  const userClient = createBrowserClient(supabaseUrl, anonKey, {
    isSingleton: false,
    cookies: {
      getAll() {
        return cookiesToSet;
      },
      setAll(cookies: AuthCookie[]) {
        cookiesToSet.splice(0, cookiesToSet.length, ...(cookies as AuthCookie[]));
      },
    },
  });

  const { data, error } = await userClient.auth.signInWithPassword(credentials);
  if (error) throw error;
  if (!data.session) throw new Error(`No session returned for ${credentials.email}`);

  const { error: setSessionError } = await userClient.auth.setSession({
    access_token: data.session.access_token,
    refresh_token: data.session.refresh_token,
  });
  if (setSessionError) throw setSessionError;

  const {
    data: { session },
    error: getSessionError,
  } = await userClient.auth.getSession();
  if (getSessionError) throw getSessionError;
  if (!session) throw new Error(`Could not read session after login for ${credentials.email}`);

  const authCookieName = `sb-${projectRef()}-auth-token`;
  const sessionCookies = cookiesToSet.filter(
    (cookie) =>
      cookie.value &&
      (cookie.name === authCookieName || cookie.name.startsWith(`${authCookieName}.`)),
  );
  if (sessionCookies.length === 0) throw new Error("Supabase SSR auth cookie was not produced.");

  await context.addCookies(
    sessionCookies.map((cookie) => ({
      name: cookie.name,
      value: cookie.value,
      domain: appHost,
      path: cookie.options.path ?? "/",
      httpOnly: cookie.options.httpOnly,
      secure: cookie.options.secure,
      sameSite: normalizeCookieSameSite(cookie.options.sameSite),
      expires: normalizeCookieExpires(cookie.options.expires),
    })),
  );
}

async function ensureRole(role: RoleName) {
  const { error } = await adminClient.from("roles").upsert({ name: role }, { onConflict: "name" });
  if (error) throw error;
}

function normalizeCookieSameSite(value: CookieOptions["sameSite"]) {
  if (value === "strict") return "Strict" as const;
  if (value === "none") return "None" as const;
  return "Lax" as const;
}

function normalizeCookieExpires(value: CookieOptions["expires"]) {
  if (typeof value === "number") return value;
  if (typeof value === "string") return Math.floor(new Date(value).getTime() / 1000);
  if (value instanceof Date) return Math.floor(value.getTime() / 1000);
  return undefined;
}

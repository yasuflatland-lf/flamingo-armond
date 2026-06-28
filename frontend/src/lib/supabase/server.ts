import "server-only";
import { createServerClient } from "@supabase/ssr";
import { cookies } from "next/headers";
import { env } from "@/env";

export async function createSupabaseServerClient() {
  const cookieStore = await cookies();
  return createServerClient(env.NEXT_PUBLIC_SUPABASE_URL, env.NEXT_PUBLIC_SUPABASE_ANON_KEY, {
    cookies: {
      getAll() {
        return cookieStore.getAll();
      },
      setAll(cookiesToSet) {
        try {
          for (const { name, value, options } of cookiesToSet) {
            cookieStore.set(name, value, options);
          }
        } catch {
          // next/headers cookies() is read-only when called from Server Component renders
          // (Next.js restriction). Middleware handles token refresh in its own mutable
          // response context. Route Handlers should not throw here; if they do this
          // catch hides a real failure — diagnose via the route's own error path.
        }
      },
    },
  });
}

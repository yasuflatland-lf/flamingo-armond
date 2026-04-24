"use client";

import { setContext } from "@apollo/client/link/context";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

// buildAuthHeaders is exported for unit-testability (Option A-variant: sibling file).
export async function buildAuthHeaders(
  headers: Record<string, string> = {},
): Promise<Record<string, string>> {
  try {
    const supabase = createSupabaseBrowserClient();
    const {
      data: { session },
    } = await supabase.auth.getSession();
    if (!session?.access_token) return headers;
    return { ...headers, authorization: `Bearer ${session.access_token}` };
  } catch {
    // Fail open: if Supabase can't return a session, send the request
    // anonymously and let the backend decide.
    // PR7 will treat missing Authorization as anonymous.
    return headers;
  }
}

export const authLink = setContext(async (_, { headers }) => ({
  headers: await buildAuthHeaders(headers as Record<string, string>),
}));

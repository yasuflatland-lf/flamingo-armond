"use client";

import { setContext } from "@apollo/client/link/context";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

// Exported separately so tests can call it directly without going through the Apollo link machinery.
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
  } catch (err) {
    // Fail open: if Supabase can't return a session, send the request anonymously
    // and let the backend decide. The backend treats missing Authorization as anonymous.
    console.warn("[authLink] getSession() failed, proceeding anonymously:", err);
    return headers;
  }
}

export const authLink = setContext(async (_, { headers }) => ({
  headers: await buildAuthHeaders(headers as Record<string, string>),
}));

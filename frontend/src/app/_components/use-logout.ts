"use client";

import { useRouter } from "next/navigation";
import { useCallback } from "react";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

/**
 * Sign the current user out, then return them to /login.
 *
 * Shared by the mobile drawer's `LogoutButton` and the desktop rail's footer
 * Logout row so the sign-out sequence is spelled once. Navigating to /login
 * *before* refresh() re-renders the layout for that URL: the HeaderSignInLink
 * suppresses itself on /login, avoiding the brief "Sign in" flash that occurs
 * when refresh() re-renders the protected page's anonymous header before its
 * server-side redirect kicks in.
 */
export function useLogout() {
  const router = useRouter();

  return useCallback(async () => {
    const supabase = createSupabaseBrowserClient();
    const { error } = await supabase.auth.signOut();
    if (error) {
      console.error("[logout] signOut failed:", error.message);
      return;
    }
    router.replace("/login");
    router.refresh();
  }, [router]);
}

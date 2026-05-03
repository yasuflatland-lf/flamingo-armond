"use client";

import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

export function LogoutButton() {
  const router = useRouter();

  async function handleLogout() {
    const supabase = createSupabaseBrowserClient();
    const { error } = await supabase.auth.signOut();
    if (error) {
      console.error("[logout] signOut failed:", error.message);
      return;
    }
    // Navigate to /login first so the layout re-renders for that URL — the
    // HeaderSignInLink suppresses itself on /login, avoiding the brief
    // "Sign in" flash that occurs when refresh() re-renders the protected
    // page's anonymous header before its server-side redirect kicks in.
    router.replace("/login");
    router.refresh();
  }

  return (
    <Button onClick={handleLogout} type="button" variant="outline" size="sm">
      Logout
    </Button>
  );
}

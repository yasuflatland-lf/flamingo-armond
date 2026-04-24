"use client";

import { Button } from "@/components/ui/button";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

export function LoginButton() {
  async function handleSignIn() {
    const supabase = createSupabaseBrowserClient();
    const { error } = await supabase.auth.signInWithOAuth({
      provider: "google",
      options: {
        redirectTo: `${window.location.origin}/auth/callback`,
      },
    });
    if (error) {
      console.error("[login] signInWithOAuth failed:", error.message);
    }
  }

  return (
    <Button onClick={handleSignIn} type="button">
      Continue with Google
    </Button>
  );
}

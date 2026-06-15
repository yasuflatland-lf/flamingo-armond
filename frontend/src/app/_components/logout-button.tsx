"use client";

import { LogOut } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

export function LogoutButton() {
  const t = useTranslations("Nav");
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
    // Quiet secondary sign-out, tuned to the drawer's nav-row rhythm (under the
    // Profile link). The desktop rail's SidebarMenuButton rows run a touch
    // tighter — the small cross-surface mismatch is intentional, not worth
    // re-spelling per surface.
    <Button
      onClick={handleLogout}
      type="button"
      variant="ghost"
      size="sm"
      className="w-full justify-start gap-3 font-normal text-foreground/70 hover:text-foreground"
    >
      <LogOut aria-hidden="true" />
      {t("logout")}
    </Button>
  );
}

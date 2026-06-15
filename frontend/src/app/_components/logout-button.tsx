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
    // Quiet, full-width utility action that sits in the nav-row rhythm of the
    // drawer / rail footer above it (the Profile link): same gap-3 / px-3 (from
    // size="sm") and hover:bg-accent. font-normal + text-foreground/70 keep it a
    // step below the Profile link's full-strength font-medium label, so it reads
    // as a secondary sign-out rather than another navigation destination — but
    // stays clearly interactive (muted-foreground sat at the AA contrast floor
    // and read closer to "disabled"). Darkens to the full foreground on hover.
    // The spacing is tuned to the drawer's gap-3 / px-3 nav rhythm; the desktop
    // rail's SidebarMenuButton rows run a few px tighter (gap-2 / p-2), so the
    // two surfaces differ slightly by design rather than re-spelling per-surface
    // classes. The leading icon is decorative (the label already names the
    // action), so it is aria-hidden.
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

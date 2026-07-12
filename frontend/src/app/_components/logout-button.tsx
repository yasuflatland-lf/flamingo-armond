"use client";

import { LogOut } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { useLogout } from "./use-logout";

export function LogoutButton() {
  const t = useTranslations("Nav");
  const logout = useLogout();

  return (
    // Quiet secondary sign-out for the mobile drawer, tuned to the drawer's
    // nav-row rhythm (px-3 / gap-3, matching the Profile link above it). The
    // desktop rail renders its own Logout row as a SidebarMenuButton, so this
    // component is drawer-only.
    <Button
      onClick={logout}
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

import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { AdminUsersClient } from "./admin-users-client";

// Admin-only route — must not be indexed.
export const metadata: Metadata = {
  title: "Users",
  robots: { index: false, follow: false },
};

export default async function AdminUsersPage() {
  // Defense-in-depth under the admin layout: redirect to / (not /login) for
  // unauthenticated or stale sessions. The admin layout is the primary gate;
  // this check is a secondary guard scoped to the users route.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/");

  return <AdminUsersClient />;
}

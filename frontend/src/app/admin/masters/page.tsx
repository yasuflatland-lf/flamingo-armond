import type { Metadata } from "next";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { AdminMastersClient } from "./admin-masters-client";

// Admin-only route — must not be indexed.
export const metadata: Metadata = {
  title: "Masters",
  robots: { index: false, follow: false },
};

export default async function AdminMastersPage() {
  // Defense-in-depth under the admin layout: redirect to / (not /login) for
  // unauthenticated or stale sessions. The admin layout is the primary gate.
  await requireAuthenticated("/");

  return <AdminMastersClient />;
}

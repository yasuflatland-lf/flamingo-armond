/**
 * Admin Dictionary Import.
 *
 * Transitional admin gate: this page does NOT enforce admin on the frontend.
 * The `validateDictionary` query and `upsertDictionary` mutation both reject
 * non-admin callers server-side with FORBIDDEN. Once issue #60 (admin layout
 * + role CRUD, which adds `frontend/src/app/admin/layout.tsx` as the unified
 * admin gate) lands, the gate moves to a server-side check on the route
 * segment layout.
 */

import { redirect } from "next/navigation";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { DictionaryImportClient } from "./dictionary-client";

export default async function AdminDictionaryPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[admin/dictionary] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  return <DictionaryImportClient />;
}

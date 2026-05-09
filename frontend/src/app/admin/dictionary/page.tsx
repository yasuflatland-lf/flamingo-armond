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
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { DictionaryImportClient } from "./dictionary-client";

export default async function AdminDictionaryPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[admin/dictionary] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  return <DictionaryImportClient />;
}

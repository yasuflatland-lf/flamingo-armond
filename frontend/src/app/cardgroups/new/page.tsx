import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/sanitize-return-to";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { NewCardgroupClient } from "./new-cardgroup-client";

interface NewCardgroupPageProps {
  searchParams: Promise<{ welcome?: string; returnTo?: string }>;
}

export default async function NewCardgroupPage({ searchParams }: NewCardgroupPageProps) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[cardgroups-new] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  const { welcome, returnTo } = await searchParams;
  const showWelcome = welcome === "1";
  const safeReturnTo = sanitizeReturnTo(returnTo);

  return <NewCardgroupClient showWelcome={showWelcome} returnTo={safeReturnTo} />;
}

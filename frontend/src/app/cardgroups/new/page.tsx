import { redirect } from "next/navigation";
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
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[cardgroups-new] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  const { welcome, returnTo } = await searchParams;
  const showWelcome = welcome === "1";
  const safeReturnTo = sanitizeReturnTo(returnTo);

  return <NewCardgroupClient showWelcome={showWelcome} returnTo={safeReturnTo} />;
}

/**
 * Open-redirect guard: allow only internal paths (must start with "/" but not
 * "//"). Rejects missing values, protocol-relative URLs ("//evil.com"), and
 * any scheme-bearing URLs ("https://...").
 */
export function sanitizeReturnTo(value: string | undefined): string | null {
  if (!value) return null;
  // reject "//" and "/\" -- both normalise to protocol-relative in browsers
  if (!value.startsWith("/") || value[1] === "/" || value[1] === "\\") return null;
  return value;
}

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/sanitize-return-to";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { NewCardgroupClient } from "./new-cardgroup-client";

interface NewCardgroupPageProps {
  searchParams: Promise<{ welcome?: string; returnTo?: string }>;
}

export default async function NewCardgroupPage({ searchParams }: NewCardgroupPageProps) {
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

  const { welcome, returnTo } = await searchParams;
  const showWelcome = welcome === "1";
  const safeReturnTo = sanitizeReturnTo(returnTo);

  return <NewCardgroupClient showWelcome={showWelcome} returnTo={safeReturnTo} />;
}

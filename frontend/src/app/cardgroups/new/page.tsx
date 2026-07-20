import { sanitizeReturnTo } from "@/lib/sanitize-return-to";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { NewCardgroupClient } from "./new-cardgroup-client";

interface NewCardgroupPageProps {
  searchParams: Promise<{ welcome?: string; returnTo?: string }>;
}

export default async function NewCardgroupPage({ searchParams }: NewCardgroupPageProps) {
  await requireAuthenticated("/login");

  const { welcome, returnTo } = await searchParams;
  const showWelcome = welcome === "1";
  const safeReturnTo = sanitizeReturnTo(returnTo);

  return <NewCardgroupClient showWelcome={showWelcome} returnTo={safeReturnTo} />;
}

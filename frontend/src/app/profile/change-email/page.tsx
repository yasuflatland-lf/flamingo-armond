import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { ChangeEmailClient } from "./change-email-client";

// Per-user private route — must not be indexed.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default async function ChangeEmailPage() {
  const auth = readAuthContext(await headers());
  if (auth.status !== "authenticated") redirect("/login");

  const t = await getTranslations("Profile");

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">{t("changeEmail")}</h1>
      <ChangeEmailClient currentEmail={auth.email} />
    </main>
  );
}

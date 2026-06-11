"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";

type Props = { className?: string };

// Suppresses itself on /login so the header does not render a self-referential
// "Sign in" link while the user is already on the sign-in page.
export function HeaderSignInLink({ className }: Props) {
  const t = useTranslations("Nav");
  const pathname = usePathname();
  if (pathname === "/login") return null;
  return (
    <Link href="/login" className={className}>
      {t("signIn")}
    </Link>
  );
}

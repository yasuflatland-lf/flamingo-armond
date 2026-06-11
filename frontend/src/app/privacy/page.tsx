import type { Metadata } from "next";
import Link from "next/link";
import { getLocale } from "next-intl/server";
import { PRIVACY_EN, PRIVACY_JA } from "./content";

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale();
  return { title: locale === "ja" ? PRIVACY_JA.title : PRIVACY_EN.title };
}

export default async function PrivacyPage() {
  const locale = await getLocale();
  const content = locale === "ja" ? PRIVACY_JA : PRIVACY_EN;

  return (
    <main className="mx-auto max-w-3xl px-6 py-12 sm:py-16">
      <nav className="mb-8">
        <Link
          href="/login"
          className="text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          ← {content.backToLogin}
        </Link>
      </nav>

      <header className="mb-10 space-y-2 border-b pb-8">
        <h1 className="text-3xl font-semibold tracking-tight">{content.title}</h1>
        <p className="text-sm text-muted-foreground">{content.lastUpdated}</p>
      </header>

      <p className="mb-10 text-sm leading-7 text-muted-foreground">{content.intro}</p>

      <div className="space-y-10">
        {content.sections.map((section) => (
          <section key={section.title}>
            <h2 className="mb-3 text-base font-semibold">{section.title}</h2>
            <p className="text-sm leading-7 text-muted-foreground">{section.body}</p>
          </section>
        ))}
      </div>
    </main>
  );
}

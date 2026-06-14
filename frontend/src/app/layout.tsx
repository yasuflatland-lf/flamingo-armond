import { SpeedInsights } from "@vercel/speed-insights/next";
import type { Metadata, Viewport } from "next";
import { Inter } from "next/font/google";
import { headers } from "next/headers";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getTranslations } from "next-intl/server";
import type { ReactNode } from "react";
import { ConditionalShell } from "@/components/conditional-shell";
import { SwRegister } from "@/components/pwa/sw-register";
import { env } from "@/env";
import { ogLocale } from "@/i18n/config";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { Providers } from "./providers";
import "./globals.css";

// Inter is the Latin UI typeface (geometric humanist sans, close to Apple's
// SF Pro). next/font self-hosts the woff2 under /_next at build time, so no CSP
// font-src change is needed. Japanese glyphs fall back to the Hiragino / Noto
// Sans JP stack declared in globals.css (`--font-sans`). Exposed as the
// `--font-inter` CSS variable so Tailwind's `--font-sans` can reference it.
const inter = Inter({ subsets: ["latin"], variable: "--font-inter", display: "swap" });

const siteTitle = "Flamingo Armond";

// Apple PWA splash screens, keyed by device dimensions. Lifted to a module const
// so generateMetadata stays readable.
const APPLE_STARTUP_IMAGES = [
  {
    url: "/splash/splash-1320x2868.png",
    media: "(device-width:440px) and (device-height:956px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-1206x2622.png",
    media: "(device-width:402px) and (device-height:874px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-1290x2796.png",
    media: "(device-width:430px) and (device-height:932px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-1179x2556.png",
    media: "(device-width:393px) and (device-height:852px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-1170x2532.png",
    media: "(device-width:390px) and (device-height:844px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-1284x2778.png",
    media: "(device-width:428px) and (device-height:926px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-1242x2688.png",
    media: "(device-width:414px) and (device-height:896px) and (-webkit-device-pixel-ratio:3)",
  },
  {
    url: "/splash/splash-828x1792.png",
    media: "(device-width:414px) and (device-height:896px) and (-webkit-device-pixel-ratio:2)",
  },
  {
    url: "/splash/splash-750x1334.png",
    media: "(device-width:375px) and (device-height:667px) and (-webkit-device-pixel-ratio:2)",
  },
  {
    url: "/splash/splash-2048x2732.png",
    media: "(device-width:1024px) and (device-height:1366px) and (-webkit-device-pixel-ratio:2)",
  },
  {
    url: "/splash/splash-1668x2388.png",
    media: "(device-width:834px) and (device-height:1194px) and (-webkit-device-pixel-ratio:2)",
  },
  {
    url: "/splash/splash-1668x2224.png",
    media: "(device-width:834px) and (device-height:1112px) and (-webkit-device-pixel-ratio:2)",
  },
  {
    url: "/splash/splash-1488x2266.png",
    media: "(device-width:744px) and (device-height:1133px) and (-webkit-device-pixel-ratio:2)",
  },
  {
    url: "/splash/splash-1620x2160.png",
    media: "(device-width:810px) and (device-height:1080px) and (-webkit-device-pixel-ratio:2)",
  },
];

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Meta");
  const locale = await getLocale();
  const siteDescription = t("description");
  return {
    metadataBase: new URL(env.NEXT_PUBLIC_SITE_URL),
    title: { default: siteTitle, template: `%s | ${siteTitle}` },
    description: siteDescription,
    // The app is auth-gated; only the public landing surface is canonical.
    alternates: { canonical: "/" },
    openGraph: {
      type: "website",
      siteName: siteTitle,
      title: siteTitle,
      description: siteDescription,
      url: "/",
      locale: ogLocale[locale],
      images: ["/opengraph-image"],
    },
    twitter: {
      card: "summary_large_image",
      title: siteTitle,
      description: siteDescription,
      images: ["/opengraph-image"],
    },
    appleWebApp: {
      capable: true,
      statusBarStyle: "default",
      title: "flamingo",
      startupImage: APPLE_STARTUP_IMAGES,
    },
  };
}

export const viewport: Viewport = {
  themeColor: "#FF6F79",
  // The app ships a single light theme. Without this, an iOS standalone PWA on a
  // device in dark mode paints the UA canvas dark, producing a black flash before
  // the first paint. Pinning the scheme to light keeps the canvas light.
  colorScheme: "light",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  const headersList = await headers();
  const nonce = headersList.get("x-nonce") ?? undefined;

  // Active locale resolved by next-intl (src/i18n/request.ts): cookie →
  // Accept-Language → "en". Drives <html lang> and the message catalog the
  // NextIntlClientProvider inherits from the server config.
  const locale = await getLocale();

  // Identity is resolved once by middleware and forwarded via request headers.
  // The shell reads it synchronously here, so there is no auth I/O at render
  // time and no async boundary, avoiding the double-load flash.
  const auth = readAuthContext(headersList);
  const shellUser = auth.status === "authenticated" ? { email: auth.email } : null;
  const isAdmin = auth.status === "authenticated" && auth.isAdmin;

  return (
    <html lang={locale} className={inter.variable}>
      {/*
        Browser extensions (ColorZilla, Grammarly, etc.) inject attributes onto
        <body> before React hydrates, which causes a benign hydration mismatch.
        suppressHydrationWarning is shallow (this element only) so real hydration
        bugs in children still surface.
      */}
      <body suppressHydrationWarning className="font-sans antialiased">
        <NextIntlClientProvider>
          <Providers nonce={nonce}>
            {/*
              ConditionalShell is a client component that re-renders on every
              client navigation via usePathname(). This hides the navigation shell
              on /login and /onboarding even when a soft navigation arrives from a
              page that SSR'd with the full shell — the root layout is a server
              component that is not re-rendered on client-side navigations, but
              ConditionalShell is. See its docblock for the full repro.
            */}
            <ConditionalShell user={shellUser} isAdmin={isAdmin}>
              {children}
            </ConditionalShell>
          </Providers>
        </NextIntlClientProvider>
        <SpeedInsights />
        <SwRegister />
      </body>
    </html>
  );
}

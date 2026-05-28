import { SpeedInsights } from "@vercel/speed-insights/next";
import type { Metadata, Viewport } from "next";
import { headers } from "next/headers";
import { type ReactNode, Suspense } from "react";
import { AuthShell } from "@/components/auth-shell";
import { BootSplash } from "@/components/boot-splash";
import { AppleInstallHint } from "@/components/pwa/apple-install-hint";
import { SwRegister } from "@/components/pwa/sw-register";
import { Providers } from "./providers";
import "./globals.css";

export const metadata: Metadata = {
  title: "flamingo-armond",
  description: "Swiping flashcard app.",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: "flamingo",
    startupImage: [
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
        media:
          "(device-width:1024px) and (device-height:1366px) and (-webkit-device-pixel-ratio:2)",
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
    ],
  },
};

export const viewport: Viewport = {
  themeColor: "#FF6F79",
  // The app ships a single light theme. Without this, an iOS standalone PWA on a
  // device in dark mode paints the UA canvas dark, producing a black flash before
  // the body/BootSplash paint. Pinning the scheme to light keeps the canvas light.
  colorScheme: "light",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  // Read the pathname forwarded by middleware so this server component can
  // decide whether to mount the navigation shell. /login renders bare so the
  // sign-in screen owns the entire viewport.
  const headersList = await headers();
  const pathname = headersList.get("x-pathname") ?? "/";
  const nonce = headersList.get("x-nonce") ?? undefined;

  if (pathname === "/login" || pathname === "/onboarding") {
    return (
      <html lang="en">
        <body suppressHydrationWarning>
          <Providers nonce={nonce}>{children}</Providers>
          <SpeedInsights />
          <SwRegister />
        </body>
      </html>
    );
  }

  return (
    <html lang="en">
      {/*
        Browser extensions (ColorZilla, Grammarly, etc.) inject attributes onto
        <body> before React hydrates, which causes a benign hydration mismatch.
        suppressHydrationWarning is shallow (this element only) so real hydration
        bugs in children still surface.
      */}
      <body suppressHydrationWarning>
        <Providers nonce={nonce}>
          <Suspense fallback={<BootSplash />}>
            <AuthShell>{children}</AuthShell>
          </Suspense>
        </Providers>
        <SpeedInsights />
        <SwRegister />
        <AppleInstallHint />
      </body>
    </html>
  );
}

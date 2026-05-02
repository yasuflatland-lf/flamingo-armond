import type { Metadata } from "next";
import type { ReactNode } from "react";
import { GlobalFAB } from "@/components/nav/global-fab";
import { GlobalHeader } from "@/components/nav/global-header";
import { Providers } from "./providers";
import "./globals.css";

export const metadata: Metadata = {
  title: "flamingo-armond",
  description: "Swiping flashcard app.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      {/*
        Browser extensions (ColorZilla, Grammarly, etc.) inject attributes onto
        <body> before React hydrates, which causes a benign hydration mismatch.
        suppressHydrationWarning is shallow (this element only) so real hydration
        bugs in children still surface.
      */}
      <body suppressHydrationWarning>
        <Providers>
          <GlobalHeader />
          {children}
          <GlobalFAB />
        </Providers>
      </body>
    </html>
  );
}

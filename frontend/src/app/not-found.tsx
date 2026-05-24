"use client";

import Link from "next/link";
import { useSuppressFab } from "@/components/nav/fab-suppression";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  // The global FAB has no meaningful "add" action on a 404; hide it while this
  // page is shown. A 404 has an arbitrary path the FAB's pathname guard cannot
  // match, so suppression is the explicit opt-out.
  useSuppressFab();

  return (
    <main className="flex min-h-[calc(100dvh-3rem)] w-full flex-col items-center justify-center gap-6 p-8 text-center">
      <div className="space-y-2">
        <p className="text-8xl font-bold tracking-tight text-brand-primary sm:text-9xl">404</p>
        <h1 className="text-2xl font-semibold tracking-tight">Oops! Page Not Found!</h1>
        <p className="mx-auto max-w-md text-sm text-muted-foreground">
          It seems like the page you're looking for does not exist or might have been removed.
        </p>
      </div>

      <Button asChild variant="brand">
        <Link href="/">Back to Home</Link>
      </Button>
    </main>
  );
}

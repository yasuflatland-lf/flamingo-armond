"use client";

import { BookOpen, Plus, User } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogoutButton } from "@/app/_components/logout-button";
import { MobileMenuTrigger } from "@/components/nav/mobile-menu-trigger";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { HeaderSignInLink } from "./header-sign-in-link";
import { ADMIN_NAV_ITEMS } from "./nav-items";

interface LogoDrawerProps {
  /** Required user record. Callers must pass a value or explicit null. */
  user: { email: string | null } | null;
  /** Required admin flag — callers must explicitly pass false for non-admins. */
  isAdmin: boolean;
}

const NAV_LINK_CLASS =
  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-accent-foreground";

export function LogoDrawer({ user, isAdmin }: LogoDrawerProps) {
  const pathname = usePathname();
  const learnMatch = pathname.match(/^\/learn\/([^/]+)$/);
  const learnCardgroupId = user && learnMatch ? learnMatch[1] : null;

  return (
    <Sheet>
      <Link
        href="/"
        aria-label="Flamingo home"
        className="rounded-md p-2 hover:bg-accent font-semibold"
      >
        🦩
      </Link>
      <div className="flex items-center gap-1">
        {learnCardgroupId && (
          <Link
            href={`/cards/new?cardgroup=${encodeURIComponent(learnCardgroupId)}&return=/learn/${encodeURIComponent(learnCardgroupId)}`}
            aria-label="Add a new card to this cardgroup"
            className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <Plus className="h-5 w-5" aria-hidden="true" />
          </Link>
        )}
        <MobileMenuTrigger />
      </div>

      <SheetContent side="left" className="flex flex-col">
        <SheetHeader>
          <SheetTitle className="sr-only">Navigation menu</SheetTitle>
        </SheetHeader>

        {user === null && pathname !== "/login" && (
          <nav className="flex flex-col gap-1">
            <SheetClose asChild>
              <HeaderSignInLink className={NAV_LINK_CLASS} />
            </SheetClose>
          </nav>
        )}

        {user && (
          <>
            <nav className="flex flex-col gap-1">
              <SheetClose asChild>
                <Link href="/cardgroups" className={NAV_LINK_CLASS}>
                  <BookOpen className="h-4 w-4 shrink-0" aria-hidden="true" />
                  Cardgroups
                </Link>
              </SheetClose>

              {isAdmin &&
                ADMIN_NAV_ITEMS.map((item) => {
                  const Icon = item.icon;
                  return (
                    <SheetClose key={item.href} asChild>
                      <Link href={item.href} className={NAV_LINK_CLASS}>
                        <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                        {item.label}
                      </Link>
                    </SheetClose>
                  );
                })}
            </nav>

            <hr className="my-3 border-t" />

            <div className="mt-auto flex flex-col gap-2" data-testid="bottom-block">
              <SheetClose asChild>
                <Link href="/profile" className={NAV_LINK_CLASS}>
                  <User className="h-4 w-4 shrink-0" aria-hidden="true" />
                  Profile
                </Link>
              </SheetClose>

              <LogoutButton />
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

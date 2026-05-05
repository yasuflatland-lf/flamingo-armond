"use client";

import { BookOpen, Settings, User } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogoutButton } from "@/app/_components/logout-button";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
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

  return (
    <Sheet>
      <SheetTrigger asChild>
        <button
          type="button"
          aria-label="Open navigation menu"
          className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring font-semibold"
        >
          🦩
        </button>
      </SheetTrigger>

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

              <SheetClose asChild>
                <Link href="/settings" className={NAV_LINK_CLASS}>
                  <Settings className="h-4 w-4 shrink-0" aria-hidden="true" />
                  Settings
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

              {user.email !== null && (
                <p className="truncate text-xs text-muted-foreground">{user.email}</p>
              )}
              <LogoutButton />
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

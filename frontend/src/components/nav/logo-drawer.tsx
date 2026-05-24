"use client";

import { BookOpen, Plus, User } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { LogoutButton } from "@/app/_components/logout-button";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
import { MobileMenuTrigger } from "@/components/nav/mobile-menu-trigger";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { resolveHeaderCreateAction } from "./header-create-action";
import { HeaderSignInLink } from "./header-sign-in-link";
import { ADMIN_NAV_ITEMS } from "./nav-items";

interface LogoDrawerProps {
  /** Required user record. Callers must pass a value or explicit null. */
  user: { email: string | null } | null;
  /** Required admin flag — callers must explicitly pass false for non-admins. */
  isAdmin: boolean;
}

const NAV_LINK_CLASS =
  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-accent-foreground active:bg-accent active:text-accent-foreground";

export function LogoDrawer({ user, isAdmin }: LogoDrawerProps) {
  const pathname = usePathname();
  const router = useRouter();
  // Hooks must run unconditionally (Rules of Hooks); only `open(...)` is called
  // conditionally inside the role branch of the click handler below.
  const { open } = useSheetSearchParam();
  // Anonymous users get no '+'; the resolver returns null for unknown routes too.
  const createAction = user ? resolveHeaderCreateAction(pathname) : null;

  // The '+' affordance dispatches a cancelable event so an in-context drawer can
  // claim the action (LearnAddCardSheet / cardgroup / card sheets listen and call
  // preventDefault). When no listener is mounted the event is uncancelled and we
  // fall back to the full-page route. The role action writes URL state directly.
  function handleCreate() {
    if (!createAction) return;
    switch (createAction.kind) {
      case "cardgroup": {
        const event = new CustomEvent("flamingo:add-cardgroup", { cancelable: true });
        if (window.dispatchEvent(event)) {
          router.push("/cardgroups/new");
        }
        return;
      }
      case "card-with-group": {
        const event = new CustomEvent("flamingo:add-card", {
          cancelable: true,
          detail: { cardgroupId: createAction.cardgroupId },
        });
        if (window.dispatchEvent(event)) {
          // `href` is already single-encoded by the resolver, including the
          // &return=/learn/... param on learn routes — push it as-is.
          router.push(createAction.href);
        }
        return;
      }
      case "role": {
        open({ mode: "new" });
        return;
      }
    }
  }

  return (
    <Sheet>
      <Link
        href="/"
        aria-label="Flamingo home"
        className="rounded-md p-2 hover:bg-accent font-semibold"
      >
        <FlamingoMark className="size-7" aria-hidden="true" />
      </Link>
      <div className="flex items-center gap-1">
        {createAction && (
          <button
            type="button"
            onClick={handleCreate}
            aria-label={createAction.label}
            className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <Plus className="h-5 w-5" aria-hidden="true" />
          </button>
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

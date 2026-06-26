"use client";

import { Import, Layers, Plus, Search } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import { LogoutButton } from "@/app/_components/logout-button";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
import { MobileMenuTrigger } from "@/components/nav/mobile-menu-trigger";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { dispatchFlamingo, FLAMINGO_EVENT, subscribeFlamingo } from "@/lib/events/flamingo-events";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { type DeckAddMenu, resolveHeaderCreateAction } from "./header-create-action";
import { resolveHeaderSearchAction } from "./header-search-action";
import { HeaderSignInLink } from "./header-sign-in-link";
import { ADMIN_NAV_ITEMS, CORE_NAV_ITEMS, FOOTER_NAV_ITEMS } from "./nav-items";

interface LogoDrawerProps {
  /** Required user record. Callers must pass a value or explicit null. */
  user: { email: string | null } | null;
  /** Required admin flag — callers must explicitly pass false for non-admins. */
  isAdmin: boolean;
}

const NAV_LINK_CLASS =
  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-accent hover:text-accent-foreground active:bg-accent active:text-accent-foreground";

export function LogoDrawer({ user, isAdmin }: LogoDrawerProps) {
  const t = useTranslations("Nav");
  const tCards = useTranslations("Cards");
  const tCardgroups = useTranslations("Cardgroups");
  const pathname = usePathname();
  const router = useRouter();
  // Hooks must run unconditionally (Rules of Hooks); only `open(...)` is called
  // conditionally inside the role branch of the click handler below.
  const { open } = useSheetSearchParam();
  // Anonymous users get no '+'; the resolver returns null for unknown routes too.
  const createAction = user ? resolveHeaderCreateAction(pathname) : null;

  const showSearch = user ? resolveHeaderSearchAction(pathname) : false;
  const searchTriggerRef = useRef<HTMLButtonElement>(null);
  const prevVisibleRef = useRef(false);
  const [searchActive, setSearchActive] = useState(false);
  const [searchVisible, setSearchVisible] = useState(false);

  // The page owns the search bar + filter state; it reports back the active
  // (filter applied) and visible (bar open) flags so the trigger can show the
  // active dot and reflect aria-expanded. When the bar closes, return focus to
  // the trigger.
  useEffect(() => {
    return subscribeFlamingo(FLAMINGO_EVENT.searchState, (detail) => {
      if (!detail) return;
      setSearchActive(detail.active);
      setSearchVisible(detail.visible);
      if (prevVisibleRef.current && !detail.visible) {
        searchTriggerRef.current?.focus();
      }
      prevVisibleRef.current = detail.visible;
    });
  }, []);

  // The '+' affordance dispatches a cancelable event so an in-context drawer can
  // claim the action (LearnAddCardSheet / cardgroup / card sheets listen and call
  // preventDefault). When no listener is mounted the event is uncancelled and we
  // fall back to the full-page route. The role and master actions write URL
  // state directly.
  function handleCreate() {
    if (!createAction) return;
    // The menu kind is rendered as a dropdown, not a click-to-dispatch button.
    if (createAction.kind === "deck-add-menu") return;
    switch (createAction.kind) {
      case "cardgroup": {
        if (dispatchFlamingo(FLAMINGO_EVENT.addCardgroup, { cancelable: true })) {
          router.push("/cardgroups/new");
        }
        return;
      }
      case "card-with-group": {
        if (
          dispatchFlamingo(FLAMINGO_EVENT.addCard, {
            cancelable: true,
            detail: { cardgroupId: createAction.cardgroupId },
          })
        ) {
          // `href` is already single-encoded by the resolver, including the
          // &return=/learn/... param on learn routes — push it as-is.
          router.push(createAction.href);
        }
        return;
      }
      // Both role and master have no separate-page target: they open the
      // create sheet by writing ?new=true to the current path. The page's own
      // useSheetSearchParam picks it up and renders its create FormSheet.
      case "role":
      case "master": {
        open({ mode: "new" });
        return;
      }
      default: {
        const _exhaustive: never = createAction;
        console.error("[LogoDrawer] unhandled createAction kind", createAction);
        return;
      }
    }
  }

  function dispatchDeckAddCard(deck: DeckAddMenu["deck"]) {
    if (deck.kind === "master") {
      dispatchFlamingo(FLAMINGO_EVENT.addMasterCard, {
        cancelable: true,
        detail: { masterId: deck.masterId },
      });
      return;
    }
    if (
      dispatchFlamingo(FLAMINGO_EVENT.addCard, {
        cancelable: true,
        detail: { cardgroupId: deck.cardgroupId },
      })
    ) {
      router.push(deck.addCardHref);
    }
  }

  function deckOwnerId(deck: DeckAddMenu["deck"]) {
    return deck.kind === "master" ? deck.masterId : deck.cardgroupId;
  }

  return (
    <Sheet>
      <Link
        href="/"
        aria-label={t("flamingoHome")}
        className="rounded-md p-2 hover:bg-accent font-semibold"
      >
        <FlamingoMark className="size-7" aria-hidden="true" />
      </Link>
      <div className="flex items-center gap-1">
        {showSearch && (
          <button
            ref={searchTriggerRef}
            type="button"
            onClick={() => dispatchFlamingo(FLAMINGO_EVENT.openSearch)}
            aria-label={t("openSearch")}
            aria-expanded={searchVisible}
            data-testid="header-search-trigger"
            className="relative rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <Search className="h-5 w-5" aria-hidden="true" />
            {searchActive && (
              <span
                aria-hidden="true"
                data-testid="header-search-active-dot"
                className="absolute right-1 top-1 h-2 w-2 rounded-full bg-brand-primary"
              />
            )}
          </button>
        )}
        {createAction &&
          (createAction.kind === "deck-add-menu" ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  aria-label={tCards("add")}
                  data-testid="header-add-menu-trigger"
                  className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
                >
                  <Plus className="h-5 w-5" aria-hidden="true" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  className="gap-2"
                  data-testid="header-add-card"
                  onSelect={() => {
                    if (createAction.kind === "deck-add-menu")
                      dispatchDeckAddCard(createAction.deck);
                  }}
                >
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  {tCards("addCard")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="gap-2"
                  data-testid="header-batch-import"
                  onSelect={() => {
                    if (createAction.kind === "deck-add-menu")
                      dispatchFlamingo(FLAMINGO_EVENT.batchImport, {
                        detail: { ownerId: deckOwnerId(createAction.deck) },
                      });
                  }}
                >
                  <Import className="h-4 w-4" aria-hidden="true" />
                  {tCardgroups("batchImport")}
                </DropdownMenuItem>
                {createAction.deck.kind === "cardgroup" && (
                  <DropdownMenuItem
                    className="gap-2"
                    data-testid="header-merge"
                    onSelect={() => {
                      if (createAction.kind === "deck-add-menu")
                        dispatchFlamingo(FLAMINGO_EVENT.merge, {
                          detail: { ownerId: deckOwnerId(createAction.deck) },
                        });
                    }}
                  >
                    <Layers className="h-4 w-4" aria-hidden="true" />
                    {tCardgroups("mergeFromCatalog")}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <button
              type="button"
              onClick={handleCreate}
              aria-label={createAction.label}
              className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <Plus className="h-5 w-5" aria-hidden="true" />
            </button>
          ))}
        <MobileMenuTrigger />
      </div>

      <SheetContent side="left" className="flex flex-col">
        <SheetHeader>
          <SheetTitle className="sr-only">{t("navigationMenu")}</SheetTitle>
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
              {CORE_NAV_ITEMS.map((item) => {
                const Icon = item.icon;
                return (
                  <SheetClose key={item.href} asChild>
                    <Link href={item.href} className={NAV_LINK_CLASS}>
                      <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                      {t(item.labelKey)}
                    </Link>
                  </SheetClose>
                );
              })}

              {isAdmin &&
                ADMIN_NAV_ITEMS.map((item) => {
                  const Icon = item.icon;
                  return (
                    <SheetClose key={item.href} asChild>
                      <Link href={item.href} className={NAV_LINK_CLASS}>
                        <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                        {t(item.labelKey)}
                      </Link>
                    </SheetClose>
                  );
                })}
            </nav>

            <hr className="my-3 border-t" />

            <div className="mt-auto flex flex-col gap-2" data-testid="bottom-block">
              {FOOTER_NAV_ITEMS.map((item) => {
                const Icon = item.icon;
                return (
                  <SheetClose key={item.href} asChild>
                    <Link href={item.href} className={NAV_LINK_CLASS}>
                      <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                      {t(item.labelKey)}
                    </Link>
                  </SheetClose>
                );
              })}

              <LogoutButton />
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

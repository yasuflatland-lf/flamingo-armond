"use client";

import { useQuery } from "@apollo/client/react";
import { Check } from "lucide-react";
import Link from "next/link";
import { MyCardgroupsQuery } from "@/app/cardgroups/queries";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedId?: string | null;
  onSelect: (cardgroupId: string) => void;
};

/**
 * Picker sheet that lists all cardgroups for the authenticated user.
 *
 * Rendered as a bottom sheet on all viewport sizes (Sheet side="bottom").
 * A max-width constraint on the inner container centres it on wider screens,
 * approximating the plan's Sheet/Dialog split without a JS media query hook.
 *
 * Accessibility: SheetContent carries role="dialog" automatically via
 * Radix Dialog primitive; SheetTitle satisfies aria-labelledby.
 */
export default function CardgroupPickerSheet({
  open,
  onOpenChange,
  selectedId,
  onSelect,
}: Props): React.ReactElement {
  const { data, loading, error, refetch } = useQuery(MyCardgroupsQuery, {
    skip: !open,
    fetchPolicy: "cache-and-network",
  });

  function handleSelect(id: string) {
    onSelect(id);
    onOpenChange(false);
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="bottom"
        className="mx-auto max-h-[70vh] w-full max-w-lg overflow-y-auto rounded-t-xl pb-safe"
        aria-labelledby="cardgroup-picker-title"
      >
        <SheetHeader className="mb-4">
          <SheetTitle id="cardgroup-picker-title">Select cardgroup</SheetTitle>
          <SheetDescription className="sr-only">
            Choose the cardgroup for this card.
          </SheetDescription>
        </SheetHeader>

        {loading && (
          <p className="py-6 text-center text-sm text-muted-foreground">
            Loading…
          </p>
        )}

        {!loading && error && (
          <div className="flex flex-col items-center gap-3 py-6 text-center">
            <p className="text-sm text-destructive">
              Failed to load cardgroups
            </p>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void refetch()}
            >
              Retry
            </Button>
          </div>
        )}

        {!loading && !error && data && (
          <>
            {data.myCardgroups.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-6 text-center">
                <p className="text-sm text-muted-foreground">
                  You don&apos;t have any cardgroups yet.
                </p>
                <Link
                  href="/cardgroups/new"
                  className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
                  onClick={() => onOpenChange(false)}
                >
                  Create cardgroup
                </Link>
              </div>
            ) : (
              <ul className="space-y-1">
                {data.myCardgroups.map((cg) => {
                  const isSelected = cg.id === selectedId;
                  return (
                    <li key={cg.id}>
                      <button
                        type="button"
                        onClick={() => handleSelect(cg.id)}
                        className={cn(
                          "flex w-full items-center justify-between rounded-md px-3 py-3 text-left text-sm",
                          "transition-colors hover:bg-accent hover:text-accent-foreground",
                          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                          isSelected && "font-medium",
                        )}
                        aria-pressed={isSelected}
                      >
                        <span className="truncate">{cg.name}</span>
                        {isSelected && (
                          <Check
                            size={16}
                            className="ml-2 flex-shrink-0 text-primary"
                            aria-hidden="true"
                          />
                        )}
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

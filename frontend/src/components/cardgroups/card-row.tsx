"use client";

import { Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import type { RefObject } from "react";
import { Button } from "@/components/ui/button";
import { SwipeableRow, type SwipeableRowHandle } from "./swipeable-row";

export type CardRowProps = {
  card: { id: string; front: string; back: string };
  rowRef: RefObject<SwipeableRowHandle | null>;
  selected: boolean;
  disabled: boolean;
  onSelectToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
};

export function CardRow({
  card,
  rowRef,
  selected,
  disabled,
  onSelectToggle,
  onEdit,
  onDelete,
}: CardRowProps) {
  const t = useTranslations("Cards");
  return (
    <SwipeableRow
      ref={rowRef}
      onDelete={onDelete}
      disabled={disabled}
      ariaLabel={t("deleteCardAriaLabel")}
    >
      <div className="group relative flex items-center justify-between gap-4 px-4 py-3 hover:bg-accent active:bg-accent transition-colors">
        {/* biome-ignore lint/a11y/noStaticElementInteractions: span is a click/keydown stopper, not an interactive element; the inner <input> is the actual control. */}
        <span
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
          className="relative z-10 shrink-0"
        >
          <input
            type="checkbox"
            className="h-4 w-4 cursor-pointer accent-primary"
            checked={selected}
            onChange={onSelectToggle}
            aria-label={t("selectCardAriaLabel")}
            data-testid={`card-select-${card.id}`}
          />
        </span>
        {/*
         * On mobile the Delete button is hidden (pointer-events-none), leaving a dead
         * non-interactive gap on the right. The after:inset-0 pseudo stretches this edit
         * target across the whole row so any tap (including that gap) opens the editor.
         * The checkbox sibling keeps relative z-10 AND pointer events because it stays an
         * active control on mobile. The Delete sibling is z-10 too (so its desktop
         * hover-reveal paints above the overlay) but pointer-events-none on mobile, so its
         * box does NOT swallow the tap — the tap falls through to the edit overlay instead
         * of being stopped by the wrapper's stopPropagation. Without this the right-hand
         * region (the Delete wrapper's box) was the one spot a row tap did NOT open the
         * editor on mobile. sm:pointer-events-auto + sm:after:content-none restore the
         * desktop behaviour: only the text column edits, Delete reveals + clicks on hover.
         */}
        {/* biome-ignore lint/a11y/useSemanticElements: a native <button> here would nest the action <button> for delete (invalid HTML); role="button" preserves screen-reader semantics without the markup conflict. */}
        <div
          role="button"
          tabIndex={0}
          onClick={onEdit}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onEdit();
            }
          }}
          className="min-w-0 flex-1 cursor-pointer space-y-1 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring after:absolute after:inset-0 after:content-[''] sm:after:content-none"
          aria-label={t("editCardAriaLabel", { front: card.front })}
          data-testid={`card-edit-target-${card.id}`}
        >
          <p className="truncate text-sm font-medium">{card.front}</p>
          <p className="truncate text-sm text-muted-foreground">{card.back}</p>
        </div>
        {/* biome-ignore lint/a11y/noStaticElementInteractions: div is a click/keydown stopper, not an interactive element; the inner Delete <button> is the actual control. */}
        <div
          className="pointer-events-none sm:pointer-events-auto relative z-10 flex shrink-0 gap-2"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Button
            variant="outline"
            size="icon"
            aria-label={t("deleteCardAriaLabel")}
            onClick={onDelete}
            data-testid={`card-delete-${card.id}`}
            className="pointer-events-none opacity-0 sm:group-hover:pointer-events-auto sm:group-hover:opacity-100 sm:group-focus-within:pointer-events-auto sm:group-focus-within:opacity-100 motion-reduce:pointer-events-auto motion-reduce:opacity-100 transition-opacity"
          >
            <Trash2 aria-hidden="true" className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </SwipeableRow>
  );
}

"use client";

import { Trash2 } from "lucide-react";
import type { RefObject } from "react";
import { SwipeableRow, type SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { Button } from "@/components/ui/button";

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
  return (
    <SwipeableRow ref={rowRef} onDelete={onDelete} disabled={disabled} ariaLabel="Delete card">
      <div className="group flex items-start justify-between gap-4 px-4 py-3 hover:bg-accent active:bg-accent transition-colors">
        {/* biome-ignore lint/a11y/noStaticElementInteractions: span is a click/keydown stopper, not an interactive element; the inner <input> is the actual control. */}
        <span
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
          className="mt-0.5 shrink-0"
        >
          <input
            type="checkbox"
            className="h-4 w-4 cursor-pointer accent-primary"
            checked={selected}
            onChange={onSelectToggle}
            aria-label="Select card"
            data-testid={`card-select-${card.id}`}
          />
        </span>
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
          className="min-w-0 flex-1 cursor-pointer space-y-1 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={`Edit card ${card.front}`}
          data-testid={`card-edit-target-${card.id}`}
        >
          <p className="text-sm font-medium">{card.front}</p>
          <p className="text-sm text-muted-foreground">{card.back}</p>
        </div>
        {/* biome-ignore lint/a11y/noStaticElementInteractions: div is a click/keydown stopper, not an interactive element; the inner Delete <button> is the actual control. */}
        <div
          className="flex shrink-0 gap-2"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Button
            variant="outline"
            size="icon"
            aria-label="Delete card"
            onClick={onDelete}
            data-testid={`card-delete-${card.id}`}
            className="opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
          >
            <Trash2 aria-hidden="true" className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </SwipeableRow>
  );
}

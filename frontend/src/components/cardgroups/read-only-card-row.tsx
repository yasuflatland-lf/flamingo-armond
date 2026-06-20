export type ReadOnlyCardRowProps = {
  card: { id: string; front: string; back: string };
};

/**
 * Presentation-only card row for the public catalog deck-detail view
 * (`/catalog/[id]`). Reuses the spacing and typography of {@link CardRow}'s text
 * content — `px-4 py-3` padding with a truncated `font-medium` front line and a
 * truncated `text-muted-foreground` back line — but carries none of the edit
 * chrome: no `SwipeableRow`, no selection checkbox, no delete button, no
 * `role="button"`/onClick. It is a pure stateless component (no hooks), so it
 * intentionally omits `"use client"`.
 */
export function ReadOnlyCardRow({ card }: ReadOnlyCardRowProps) {
  return (
    <div className="px-4 py-3" data-testid={`read-only-card-${card.id}`}>
      <p className="truncate text-sm font-medium">{card.front}</p>
      <p className="truncate text-sm text-muted-foreground">{card.back}</p>
    </div>
  );
}

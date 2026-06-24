export type ReadOnlyCardRowProps = {
  card: { id: string; front: string; back: string };
};

/**
 * Presentation-only card row for the public catalog deck-detail view
 * (`/catalog/[id]`). Each row is a discrete bordered card — `rounded-lg border
 * border-border bg-background` matching {@link CardgroupListItem}'s card chrome —
 * so the list reads as gapped cards (the consumer's `<ul>` supplies the
 * inter-card `space-y-3`) rather than a single divided table. It reuses
 * {@link CardRow}'s text spacing and typography (`px-4 py-3` padding, a truncated
 * `font-medium` front line, a truncated `text-muted-foreground` back line) but
 * carries none of the edit chrome: no `SwipeableRow`, no selection checkbox, no
 * delete button, no `role="button"`/onClick (it is non-interactive — there is no
 * card-detail route to navigate to). It is a pure stateless component (no hooks),
 * so it intentionally omits `"use client"`.
 */
export function ReadOnlyCardRow({ card }: ReadOnlyCardRowProps) {
  return (
    <div
      className="rounded-lg border border-border bg-background px-4 py-3"
      data-testid={`read-only-card-${card.id}`}
    >
      <p className="truncate text-sm font-medium">{card.front}</p>
      <p className="truncate text-sm text-muted-foreground">{card.back}</p>
    </div>
  );
}

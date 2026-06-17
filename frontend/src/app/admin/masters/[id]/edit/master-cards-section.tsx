/**
 * Placeholder for the master-cards editing UI. #522 replaces the body with the
 * paginated cards connection and extends the props with
 * `initialEdges` / `initialPageInfo` / `initialTotalCount`. The `masterId` prop
 * name is the stable interface this file exposes — do not rename it.
 */
export function MasterCardsSection({ masterId }: { masterId: string }) {
  return <section data-testid="master-cards-section" data-master-id={masterId} />;
}

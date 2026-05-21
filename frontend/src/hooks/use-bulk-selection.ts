import { useCallback, useMemo, useState } from "react";

export interface UseBulkSelectionResult<TId> {
  selectedIds: Set<TId>;
  toggleSelected: (id: TId) => void;
  clearSelection: () => void;
  isSelected: (id: TId) => boolean;
  count: number;
}

// Generic Set-of-IDs selection hook with stable callback identities.
export function useBulkSelection<TId = string>(): UseBulkSelectionResult<TId> {
  const [selectedIds, setSelectedIds] = useState<Set<TId>>(new Set());

  const toggleSelected = useCallback((id: TId) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  const isSelected = useCallback(
    (id: TId) => selectedIds.has(id),
    [selectedIds]
  );

  const count = useMemo(() => selectedIds.size, [selectedIds]);

  return {
    selectedIds,
    toggleSelected,
    clearSelection,
    isSelected,
    count,
  };
}

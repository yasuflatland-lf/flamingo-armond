import { useCallback, useState } from "react";

export interface UseBulkSelectionResult<TId extends PropertyKey> {
  selectedIds: ReadonlySet<TId>;
  toggleSelected: (id: TId) => void;
  clearSelection: () => void;
  isSelected: (id: TId) => boolean;
  count: number;
}

/** Generic Set-of-IDs selection hook (Set<TId> with toggle/clear/has/count). */
export function useBulkSelection<TId extends PropertyKey = string>(): UseBulkSelectionResult<TId> {
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

  const isSelected = useCallback((id: TId) => selectedIds.has(id), [selectedIds]);

  const count = selectedIds.size;

  return {
    selectedIds,
    toggleSelected,
    clearSelection,
    isSelected,
    count,
  };
}

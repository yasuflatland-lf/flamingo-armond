import { useEffect } from "react";

/**
 * Bridges a TanStack Form `isDirty` subscription value to a parent callback
 * (e.g. to drive FormSheet's discard guard). Renders nothing.
 */
export function DirtyStateBridge({
  dirty,
  onDirtyChange,
}: {
  dirty: boolean;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  useEffect(() => {
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  return null;
}

import { useCallback, useEffect, useState } from "react";

export interface UseDebouncedSearchInput {
  delayMs?: number; // default 300
}

export interface UseDebouncedSearchResult {
  input: string; // raw input (immediate)
  query: string | null; // debounced + trimmed; null when input.trim() is empty
  setInput: (next: string) => void;
  clear: () => void;
}

// Hook that splits raw input (updated synchronously) from the debounced,
// trimmed query value. The "immediate-reset" effect for caller-side IO state
// (fetchingRef, fetchMoreError) belongs in the caller hook, not here.
// See docs/pagination/split-debounce-from-immediate-reset.md.
export function useDebouncedSearch(
  opts?: UseDebouncedSearchInput
): UseDebouncedSearchResult {
  const delayMs = opts?.delayMs ?? 300;

  const [input, setStateInput] = useState<string>("");
  const [query, setQuery] = useState<string | null>(null);

  useEffect(() => {
    const id = setTimeout(() => {
      const trimmed = input.trim();
      setQuery(trimmed !== "" ? trimmed : null);
    }, delayMs);
    return () => {
      clearTimeout(id);
    };
  }, [input, delayMs]);

  const setInput = useCallback((next: string) => {
    setStateInput(next);
  }, []);

  const clear = useCallback(() => {
    setStateInput("");
    setQuery(null);
  }, []);

  return { input, query, setInput, clear };
}

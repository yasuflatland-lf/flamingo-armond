"use client";

import { ArrowLeft, Search, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useRef } from "react";

interface SearchTakeoverBarProps {
  /** Whether the bar is shown. When false the component renders nothing. */
  open: boolean;
  /** Current (immediate) input value. */
  value: string;
  /** Called on every keystroke with the new value. */
  onChange: (value: string) => void;
  /** Clears the field (resets the underlying query). */
  onClear: () => void;
  /** Closes the bar (back button / Escape). */
  onClose: () => void;
  /** Field placeholder — caller-specific copy. */
  placeholder: string;
  /** Field aria-label — caller-specific copy. */
  ariaLabel: string;
}

/**
 * Full-width search bar that slides over the mobile global header
 * ("header takeover"). Mobile only (`md:hidden`). Presentational: it owns no
 * filter state — the caller wires `value`/`onChange` to its
 * `useDebouncedSearch` instance. Used by `/cardgroups`; built to extend to
 * other list screens.
 */
export function SearchTakeoverBar({
  open,
  value,
  onChange,
  onClear,
  onClose,
  placeholder,
  ariaLabel,
}: SearchTakeoverBarProps) {
  const t = useTranslations("Search");
  const inputRef = useRef<HTMLInputElement>(null);

  // Move focus into the field when the bar opens.
  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  // Close on Escape even if focus has left the input (e.g. user scrolled
  // results). Window-level so the key works regardless of focus target.
  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return (
    // biome-ignore lint/a11y/useSemanticElements: the <search> element is not yet mapped to role "search" by the installed aria-query, breaking getByRole("search") in vitest; role="search" on a div is a valid ARIA landmark.
    <div
      role="search"
      data-testid="search-takeover"
      className="fixed inset-x-0 top-0 z-50 flex h-14 items-center gap-2 border-b bg-background px-3 md:hidden motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-top-2 motion-safe:duration-200"
    >
      <button
        type="button"
        onClick={onClose}
        aria-label={t("back")}
        data-testid="search-takeover-close"
        className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
      >
        <ArrowLeft className="h-5 w-5" aria-hidden="true" />
      </button>
      <div className="relative flex-1">
        <Search
          className="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <input
          ref={inputRef}
          type="search"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          aria-label={ariaLabel}
          data-testid="search-takeover-input"
          className="w-full rounded-md border border-input bg-background py-2.5 pl-8 pr-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      </div>
      {value !== "" && (
        <button
          type="button"
          onClick={onClear}
          aria-label={t("clear")}
          className="rounded-md p-2 hover:bg-accent focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <X className="h-5 w-5" aria-hidden="true" />
        </button>
      )}
    </div>
  );
}

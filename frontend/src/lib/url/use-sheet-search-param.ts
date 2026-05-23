"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo } from "react";

/**
 * URL-backed sheet open/close state for listing pages.
 *
 * URL contract:
 *   - `?new=true` → `{ mode: "new" }` (Create sheet)
 *   - `?edit=<id>` → `{ mode: "edit", id }` (Edit sheet for entity `<id>`)
 *   - neither    → `{ mode: "closed" }`
 *
 * The two sheet keys are mutually exclusive by construction:
 * `open()` always strips the opposing key before writing the new one.
 *
 * Singleton-resource pages (e.g. `/profile`, which has no per-entity id) may
 * pass a stable non-empty sentinel id like `"self"` and branch on it at the
 * consumer. Choose a sentinel that does not collide with the entity-id space
 * actually used elsewhere on the same surface.
 */
export type SheetState = { mode: "closed" } | { mode: "new" } | { mode: "edit"; id: string };

type OpenSheetState = Exclude<SheetState, { mode: "closed" }>;

type CloseOptions = {
  /** Trigger a Next.js RSC re-fetch after closing (e.g. after a mutation). */
  refresh?: boolean;
};

/**
 * Map of mode → URL search-param name. Keeping this in one place ensures the
 * parser, the writer, and the strip-opposing-param step stay in sync when a
 * new mode is added.
 */
const SHEET_PARAM_BY_MODE = {
  new: "new",
  edit: "edit",
} as const satisfies Record<Exclude<SheetState["mode"], "closed">, string>;

const SHEET_PARAM_NAMES = Object.values(SHEET_PARAM_BY_MODE);

function buildHref(pathname: string, params: URLSearchParams) {
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

function cloneWithoutSheetParams(searchParams: URLSearchParams) {
  const params = new URLSearchParams(searchParams.toString());
  for (const name of SHEET_PARAM_NAMES) {
    params.delete(name);
  }
  return params;
}

export function useSheetSearchParam(): {
  state: SheetState;
  open: (next: OpenSheetState) => void;
  close: (options?: CloseOptions) => void;
} {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();

  const state = useMemo<SheetState>(() => {
    if (searchParams.get(SHEET_PARAM_BY_MODE.new) === "true") {
      return { mode: "new" };
    }

    const editId = searchParams.get(SHEET_PARAM_BY_MODE.edit);
    // `?edit=` (empty value) is treated as closed, not as a malformed edit
    // request — without this guard the truthy check `if (editId)` lets it fall
    // through and the sheet silently fails to open with no observable signal.
    if (editId === "") {
      console.warn("[useSheetSearchParam] ignoring empty ?edit= value");
      return { mode: "closed" };
    }
    if (editId !== null) {
      return { mode: "edit", id: editId };
    }

    return { mode: "closed" };
  }, [searchParams]);

  const open = useCallback(
    (next: OpenSheetState) => {
      const params = cloneWithoutSheetParams(searchParams);

      if (next.mode === "new") {
        params.set(SHEET_PARAM_BY_MODE.new, "true");
      } else {
        if (next.id === "") {
          throw new Error(
            "useSheetSearchParam.open: edit mode requires a non-empty id (got empty string)",
          );
        }
        params.set(SHEET_PARAM_BY_MODE.edit, next.id);
      }

      router.push(buildHref(pathname, params), { scroll: false });
    },
    [pathname, router, searchParams],
  );

  const close = useCallback(
    (options: CloseOptions = {}) => {
      const params = cloneWithoutSheetParams(searchParams);

      router.replace(buildHref(pathname, params), { scroll: false });
      if (options.refresh === true) {
        router.refresh();
      }
    },
    [pathname, router, searchParams],
  );

  return { state, open, close };
}

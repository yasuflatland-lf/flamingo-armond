"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo } from "react";

export type SheetState = { mode: "closed" } | { mode: "new" } | { mode: "edit"; id: string };

type OpenSheetState = Exclude<SheetState, { mode: "closed" }>;

type CloseOptions = {
  refresh?: boolean;
};

function buildHref(pathname: string, params: URLSearchParams) {
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

function cloneWithoutSheetParams(searchParams: URLSearchParams) {
  const params = new URLSearchParams(searchParams.toString());
  params.delete("edit");
  params.delete("new");
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
    if (searchParams.get("new") === "true") {
      return { mode: "new" };
    }

    const editId = searchParams.get("edit");
    if (editId) {
      return { mode: "edit", id: editId };
    }

    return { mode: "closed" };
  }, [searchParams]);

  const open = useCallback(
    (next: OpenSheetState) => {
      const params = cloneWithoutSheetParams(searchParams);

      if (next.mode === "new") {
        params.set("new", "true");
      } else {
        params.set("edit", next.id);
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

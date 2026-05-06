"use client";

import * as React from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type PaginationState,
  type Updater,
} from "@tanstack/react-table";

import { DataTablePagination } from "@/components/ui/data-table-pagination";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

import type { AdminUserRow } from "./users-columns";

export interface UsersTableProps {
  columns: ColumnDef<AdminUserRow>[];
  data: AdminUserRow[];
  /** 0-based index of the currently visible page. Owned by the parent. */
  pageIndex: number;
  /** Number of rows per page. Owned by the parent. */
  pageSize: number;
  /** Server-reported total number of matching users (used for page count). */
  totalCount: number;
  /** Called with the next 0-based page index when the user navigates pages. */
  onPageChange: (next: number) => void;
  /** Called with the new page size when the user changes rows-per-page. */
  onPageSizeChange: (size: number) => void;
  /** When true and data is empty, renders a "Loading…" placeholder row. */
  isLoading: boolean;
}

/**
 * Presentational DataTable shell for the admin /users page.
 *
 * Renders a TanStack Table with manual (server-driven) pagination and a
 * DataTablePagination control below the table. This component does NOT fetch
 * data, manage URL state, or own page-index / page-size state — the parent
 * is responsible for all of that. The toolbar lives outside this component,
 * in the page-shell's toolbar slot.
 *
 * Pagination is cursor-based on the server side; the parent translates
 * 0-based page indices to cursors. See `.claude/rules/pagination.md`.
 */
export function UsersTable({
  columns,
  data,
  pageIndex,
  pageSize,
  totalCount,
  onPageChange,
  onPageSizeChange,
  isLoading,
}: UsersTableProps): React.ReactElement {
  const totalPages = Math.ceil(totalCount / pageSize);

  /**
   * Translate TanStack Table's pagination updater back to the parent's
   * onPageChange / onPageSizeChange callbacks. TanStack passes either a new
   * PaginationState object or an updater function; resolve both forms before
   * comparing fields so the callbacks are only invoked when something actually
   * changed.
   */
  function handlePaginationChange(updater: Updater<PaginationState>): void {
    const current: PaginationState = { pageIndex, pageSize };
    const next = typeof updater === "function" ? updater(current) : updater;
    if (next.pageIndex !== current.pageIndex) {
      onPageChange(next.pageIndex);
    }
    if (next.pageSize !== current.pageSize) {
      onPageSizeChange(next.pageSize);
    }
  }

  const table = useReactTable<AdminUserRow>({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    // Pagination is server-driven (cursor-based). The parent owns cursors and
    // translates 0-based page indices to after/before cursor values.
    manualPagination: true,
    pageCount: totalPages,
    state: {
      pagination: { pageIndex, pageSize },
    },
    onPaginationChange: handlePaginationChange,
  });

  return (
    <div className="flex flex-col gap-4">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead key={header.id}>
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>

        <TableBody>
          {data.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columns.length} className="h-24 text-center">
                {isLoading ? "Loading…" : "No users."}
              </TableCell>
            </TableRow>
          ) : (
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id} data-state={row.getIsSelected() ? "selected" : undefined}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      <DataTablePagination table={table} totalPages={totalPages} />
    </div>
  );
}

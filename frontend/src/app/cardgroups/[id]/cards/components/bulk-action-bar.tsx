"use client";

import { Trash2, X } from "lucide-react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";

export type BulkActionBarProps = {
  count: number;
  busy: boolean;
  onConfirm: () => void;
  onClear: () => void;
};

export function BulkActionBar({ count, busy, onConfirm, onClear }: BulkActionBarProps) {
  return (
    <div
      className="mb-3 flex items-center gap-3 rounded-md border border-border bg-muted/50 px-4 py-2"
      data-testid="cards-bulk-action-bar"
    >
      <span className="flex-1 text-sm font-medium">{count} selected</span>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            disabled={busy}
            data-testid="cards-bulk-delete-button"
          >
            Delete selected
            <Trash2 aria-hidden="true" className="ml-1.5 h-4 w-4" />
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {count} cards?</AlertDialogTitle>
            <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction data-testid="cards-bulk-confirm" onClick={onConfirm}>
              Delete {count}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <Button variant="outline" size="sm" onClick={onClear}>
        Cancel
        <X aria-hidden="true" className="ml-1.5 h-4 w-4" />
      </Button>
    </div>
  );
}

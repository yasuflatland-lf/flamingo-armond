"use client";

import * as React from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from "@/components/ui/drawer";
import { useIsMobile } from "@/hooks/use-mobile";

type FormSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  dirty?: boolean;
  confirmOnDismiss?: boolean;
  confirmMessage?: string;
  submitting?: boolean;
  children: React.ReactNode;
};

const FormSheetCloseContext = React.createContext<(() => void) | null>(null);

function useFormSheetClose() {
  const close = React.useContext(FormSheetCloseContext);

  if (close === null) {
    throw new Error("useFormSheetClose must be used within a FormSheet");
  }

  return close;
}

function FormSheet({
  open,
  onOpenChange,
  title,
  dirty = false,
  confirmOnDismiss = false,
  confirmMessage = "Your unsaved changes will be lost.",
  submitting = false,
  children,
}: FormSheetProps) {
  const isMobile = useIsMobile();
  const [confirmOpen, setConfirmOpen] = React.useState(false);

  const requestOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      if (nextOpen) {
        onOpenChange(true);
        return;
      }

      if (submitting) {
        return;
      }

      if (confirmOnDismiss && dirty) {
        setConfirmOpen(true);
        return;
      }

      onOpenChange(false);
    },
    [confirmOnDismiss, dirty, onOpenChange, submitting],
  );

  const close = React.useCallback(() => {
    requestOpenChange(false);
  }, [requestOpenChange]);

  const discard = React.useCallback(() => {
    if (submitting) {
      return;
    }

    setConfirmOpen(false);
    onOpenChange(false);
  }, [onOpenChange, submitting]);

  React.useEffect(() => {
    if (!open) {
      setConfirmOpen(false);
    }
  }, [open]);

  const body = isMobile ? (
    <Drawer open={open} onOpenChange={requestOpenChange}>
      <DrawerContent aria-describedby={undefined}>
        <DrawerHeader>
          <DrawerTitle>{title}</DrawerTitle>
        </DrawerHeader>
        <div
          className="flex max-h-[calc(100dvh-7rem)] flex-col gap-4 overflow-y-auto px-4 pb-4"
          data-testid="form-sheet-body"
        >
          {children}
        </div>
      </DrawerContent>
    </Drawer>
  ) : (
    <Dialog open={open} onOpenChange={requestOpenChange}>
      <DialogContent aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  );

  return (
    <FormSheetCloseContext.Provider value={close}>
      {body}
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard your changes?</AlertDialogTitle>
            <AlertDialogDescription>{confirmMessage}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep editing</AlertDialogCancel>
            <AlertDialogAction disabled={submitting} onClick={discard}>
              Discard
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </FormSheetCloseContext.Provider>
  );
}

export { FormSheet, useFormSheetClose };

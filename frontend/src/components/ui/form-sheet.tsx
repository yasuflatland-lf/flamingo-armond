"use client";

import { useTranslations } from "next-intl";
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
import { buttonVariants } from "@/components/ui/button";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";

type FormSheetSize = "sm" | "md" | "lg";

const desktopWidthBySize: Record<FormSheetSize, string> = {
  sm: "sm:max-w-md",
  md: "sm:max-w-lg",
  lg: "sm:max-w-2xl",
};

type FormSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string | React.ReactNode;
  description?: string;
  dirty?: boolean;
  confirmOnDismiss?: boolean;
  confirmMessage?: string;
  submitting?: boolean;
  size?: FormSheetSize;
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
  description,
  dirty = false,
  confirmOnDismiss = false,
  confirmMessage = "Your unsaved changes will be lost.",
  submitting = false,
  size = "md",
  children,
}: FormSheetProps) {
  const t = useTranslations("Common");
  const a11yDescription = description ?? (typeof title === "string" ? `${title} form` : "Form");
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
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle className="overflow-hidden text-ellipsis whitespace-nowrap">
            {title}
          </DrawerTitle>
          <DrawerDescription className={description ? undefined : "sr-only"}>
            {a11yDescription}
          </DrawerDescription>
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
    <Sheet open={open} onOpenChange={requestOpenChange}>
      <SheetContent side="right" className={cn("flex w-full flex-col", desktopWidthBySize[size])}>
        <SheetHeader>
          <SheetTitle className="overflow-hidden text-ellipsis whitespace-nowrap">
            {title}
          </SheetTitle>
          <SheetDescription className={description ? undefined : "sr-only"}>
            {a11yDescription}
          </SheetDescription>
        </SheetHeader>
        <div className="-mx-2 flex-1 overflow-y-auto px-2" data-testid="form-sheet-body">
          {children}
        </div>
      </SheetContent>
    </Sheet>
  );

  return (
    <FormSheetCloseContext.Provider value={close}>
      {body}
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent className="z-[60]">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("discardChanges")}</AlertDialogTitle>
            <AlertDialogDescription>{confirmMessage}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("keepEditing")}</AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: "destructive" })}
              disabled={submitting}
              onClick={discard}
            >
              {t("discard")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </FormSheetCloseContext.Provider>
  );
}

export { FormSheet, useFormSheetClose };

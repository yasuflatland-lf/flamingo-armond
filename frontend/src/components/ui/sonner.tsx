"use client";

import { CircleCheck, Info, LoaderCircle, OctagonX, TriangleAlert } from "lucide-react";
import { Toaster as Sonner } from "sonner";

type ToasterProps = React.ComponentProps<typeof Sonner>;

// Default ("normal") toast styling is driven through sonner's own CSS custom
// properties, not Tailwind `classNames`: sonner's [data-sonner-toast][data-styled]
// rules out-specify any utility class, so utility-class colours never take effect.
// Repointing the normal-* variables turns the delete/undo popup into a deep
// brand-coral accent surface with legible white text, and the Undo pill inherits
// the inverse (white fill, coral text) from sonner's [data-button] rule.
// Rich-colour typed toasts (success/error/…) read their own --success-*/--error-*
// vars, so this only recolours the default toast.
const brandToastStyle = {
  "--normal-bg": "var(--brand-primary-strong)",
  "--normal-text": "var(--brand-primary-foreground)",
  "--normal-border": "var(--brand-primary-strong)",
  // Close button sits on sonner's neutral gray ramp; repoint glyph/border/hover at
  // translucent white so the "✕" stays visible on the coral surface.
  "--gray12": "var(--brand-primary-foreground)",
  "--gray4": "color-mix(in oklch, var(--brand-primary-foreground) 35%, transparent)",
  "--gray2": "color-mix(in oklch, var(--brand-primary-foreground) 16%, transparent)",
  "--gray5": "color-mix(in oklch, var(--brand-primary-foreground) 45%, transparent)",
} as React.CSSProperties;

// The app ships a single light theme (no ThemeProvider is mounted). Pin sonner to
// "light"; leaving it on "system" let the toast follow the OS and render black on
// dark-mode machines while the rest of the UI stayed light.
const Toaster = ({ richColors = true, closeButton = true, ...props }: ToasterProps) => {
  return (
    <Sonner
      theme="light"
      richColors={richColors}
      closeButton={closeButton}
      className="toaster group"
      icons={{
        success: <CircleCheck className="h-4 w-4" />,
        info: <Info className="h-4 w-4" />,
        warning: <TriangleAlert className="h-4 w-4" />,
        error: <OctagonX className="h-4 w-4" />,
        loading: <LoaderCircle className="h-4 w-4 animate-spin" />,
      }}
      toastOptions={{
        style: brandToastStyle,
        classNames: {
          toast: "group toast group-[.toaster]:shadow-lg",
          cancelButton: "group-[.toast]:bg-muted group-[.toast]:text-muted-foreground",
        },
      }}
      {...props}
    />
  );
};

export { Toaster };

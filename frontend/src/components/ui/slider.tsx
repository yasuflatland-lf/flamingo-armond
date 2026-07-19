"use client";

import * as SliderPrimitive from "@radix-ui/react-slider";
import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * shadcn/Radix single-thumb Slider.
 *
 * The filled `Range` uses the coral brand-fill token (`--brand-primary`, exposed
 * to Tailwind as the `brand-primary` color, i.e. the same fill the primary CTA
 * uses); the thumb is white with a coral border and a `--ring` focus ring. Arrow
 * keys move the value by `step` (Radix built-in).
 *
 * Consumers supply `aria-label` / `aria-valuetext` / `aria-describedby`; all
 * three are forwarded to the thumb — the element Radix marks `role="slider"` —
 * because Radix spreads the remaining props onto the root and only reads
 * `aria-label` off the thumb. An `aria-describedby` left on the root would name
 * a description for an element assistive tech never lands on, so forwarding here
 * is what lets a screen reader announce the value text and any attached note.
 */
const Slider = React.forwardRef<
  React.ElementRef<typeof SliderPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof SliderPrimitive.Root>
>(
  (
    {
      className,
      "aria-label": ariaLabel,
      "aria-valuetext": ariaValueText,
      "aria-describedby": ariaDescribedBy,
      ...props
    },
    ref,
  ) => (
    <SliderPrimitive.Root
      ref={ref}
      className={cn(
        "relative flex w-full touch-none select-none items-center data-[disabled]:opacity-50",
        className,
      )}
      {...props}
    >
      <SliderPrimitive.Track className="relative h-2 w-full grow overflow-hidden rounded-full bg-muted">
        <SliderPrimitive.Range className="absolute h-full bg-brand-primary" />
      </SliderPrimitive.Track>
      <SliderPrimitive.Thumb
        aria-label={ariaLabel}
        aria-valuetext={ariaValueText}
        aria-describedby={ariaDescribedBy}
        className="block h-5 w-5 rounded-full border-2 border-brand-primary bg-white shadow ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50"
      />
    </SliderPrimitive.Root>
  ),
);
Slider.displayName = SliderPrimitive.Root.displayName;

export { Slider };

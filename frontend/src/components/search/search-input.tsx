import { Search } from "lucide-react";
import * as React from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export interface SearchInputProps extends Omit<React.ComponentPropsWithoutRef<"input">, "type"> {
  /**
   * Renders a leading search icon inside the field and shifts the left
   * padding to make room for it. Off by default — most desktop search boxes
   * in this app render bare, matching `admin-list-search.tsx`.
   */
  icon?: boolean;
}

/**
 * Design-system search field: a design-system `<Input type="search">` with
 * an optional leading icon. Every desktop search box in the app (catalog,
 * cardgroups, merge-from-catalog, cards) renders through this component so
 * they share one focus-ring / sizing / placeholder-color source instead of
 * a copy-pasted className string.
 */
export const SearchInput = React.forwardRef<HTMLInputElement, SearchInputProps>(
  ({ icon = false, className, ...props }, ref) => {
    if (!icon) {
      return <Input type="search" className={className} ref={ref} {...props} />;
    }
    return (
      <div className="relative">
        <Search
          aria-hidden="true"
          className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
        />
        <Input type="search" className={cn("pl-8", className)} ref={ref} {...props} />
      </div>
    );
  },
);
SearchInput.displayName = "SearchInput";

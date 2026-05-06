"use client";

import { Check, PlusCircle } from "lucide-react";
import type * as React from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

export interface UsersToolbarProps {
  searchInput: string;
  onSearchInputChange: (value: string) => void;
  /**
   * The selected role id, or null when no role filter is active.
   * Must be explicitly passed as null (not omitted) so the caller
   * always acknowledges the filter state.
   */
  roleFilterValue: string | null;
  onRoleFilterChange: (roleId: string | null) => void;
  availableRoles: { id: string; name: string }[];
}

/**
 * Toolbar for the admin /users listing page.
 *
 * Renders a controlled search input (debouncing is the parent's responsibility)
 * and a single-select role faceted filter built with Popover + Command.
 * The DataTableFacetedFilter primitive is not reused here because it is tightly
 * coupled to @tanstack/react-table's Column API, which this page does not use.
 */
export function UsersToolbar({
  searchInput,
  onSearchInputChange,
  roleFilterValue,
  onRoleFilterChange,
  availableRoles,
}: UsersToolbarProps): React.ReactElement {
  const selectedRole = availableRoles.find((r) => r.id === roleFilterValue) ?? null;

  return (
    <div className="mb-6 flex items-center gap-3">
      {/* Search input — grows to fill available space */}
      <input
        type="search"
        placeholder="Filter users..."
        value={searchInput}
        onChange={(e) => onSearchInputChange(e.target.value)}
        className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-label="Filter users"
      />

      {/* Role faceted filter — single-select via Popover + Command */}
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="outline" size="sm" className="h-9 border-dashed">
            <PlusCircle className="mr-2 h-4 w-4" />
            Role
            {selectedRole !== null && (
              <>
                <Separator orientation="vertical" className="mx-2 h-4" />
                <Badge variant="secondary" className="rounded-sm px-1 font-normal">
                  {selectedRole.name}
                </Badge>
              </>
            )}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[200px] p-0" align="start">
          <Command>
            <CommandInput placeholder="Role" />
            <CommandList>
              <CommandEmpty>No results found.</CommandEmpty>
              <CommandGroup>
                {availableRoles.map((role) => {
                  const isSelected = role.id === roleFilterValue;
                  return (
                    <CommandItem
                      key={role.id}
                      onSelect={() => {
                        // Single-select: toggle off when already selected.
                        onRoleFilterChange(isSelected ? null : role.id);
                      }}
                    >
                      <div
                        className={cn(
                          "mr-2 flex h-4 w-4 items-center justify-center rounded-sm border border-primary",
                          isSelected
                            ? "bg-primary text-primary-foreground"
                            : "opacity-50 [&_svg]:invisible",
                        )}
                      >
                        <Check className="h-4 w-4" />
                      </div>
                      <span>{role.name}</span>
                    </CommandItem>
                  );
                })}
              </CommandGroup>
              {roleFilterValue !== null && (
                <>
                  <CommandSeparator />
                  <CommandGroup>
                    <CommandItem
                      onSelect={() => onRoleFilterChange(null)}
                      className="justify-center text-center"
                    >
                      Clear filter
                    </CommandItem>
                  </CommandGroup>
                </>
              )}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  );
}

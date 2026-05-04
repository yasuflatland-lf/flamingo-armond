"use client";

import { User } from "lucide-react";
import { LogoutButton } from "@/app/_components/logout-button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

interface AvatarPopoverProps {
  /** Required email address. Callers must pass a value or explicit null. */
  email: string;
}

export function AvatarPopover({ email }: AvatarPopoverProps) {
  const initial = email.length > 0 ? email[0]!.toUpperCase() : null;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label="Open account menu"
          className="flex items-center justify-center h-8 w-8 rounded-full bg-accent text-accent-foreground hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
        >
          {initial !== null ? (
            <span className="text-sm font-medium leading-none">{initial}</span>
          ) : (
            <User className="h-4 w-4" aria-hidden="true" />
          )}
        </button>
      </PopoverTrigger>

      <PopoverContent align="end" side="top" className="w-56 p-2">
        {/* Non-interactive email row — shows identity only, no PII beyond address */}
        <div className="text-sm text-muted-foreground px-2 py-1.5 truncate">{email}</div>

        <div className="mt-1">
          <LogoutButton />
        </div>
      </PopoverContent>
    </Popover>
  );
}

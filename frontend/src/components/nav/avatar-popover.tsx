"use client";

import { User } from "lucide-react";
import { LogoutButton } from "@/app/_components/logout-button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

interface AvatarPopoverProps {
  /**
   * The authenticated user's email address, or null when the email is not
   * available. Required — callers must pass either a real email string or null
   * explicitly (do not omit).
   */
  email: string | null;
}

export function AvatarPopover({ email }: AvatarPopoverProps) {
  const initial =
    email !== null && email.length > 0 ? email[0]!.toUpperCase() : null;

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
        {/* Non-interactive email row — omitted entirely when email is not available */}
        {email !== null && (
          <div className="text-sm text-muted-foreground px-2 py-1.5 truncate">{email}</div>
        )}

        <div className="mt-1">
          <LogoutButton />
        </div>
      </PopoverContent>
    </Popover>
  );
}

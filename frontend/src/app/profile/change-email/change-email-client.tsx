"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

type Props = {
  /**
   * The user's current email address. Required — callers must pass the value
   * or explicit null; never collapse to "". Per
   * `.claude/rules/frontend-typescript-conventions.md` § "Required `string |
   * null` over optional".
   */
  currentEmail: string | null;
};

function classifyUpdateUserError(message: string): string {
  const lower = message.toLowerCase();
  if (lower.includes("rate limit")) {
    return "Too many requests. Please wait a moment and try again.";
  }
  if (lower.includes("already registered")) {
    return "That email address is already in use.";
  }
  return message;
}

export function ChangeEmailClient({ currentEmail }: Props) {
  const [newEmail, setNewEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    e.stopPropagation();
    setLoading(true);
    setError(null);
    try {
      const supabase = createSupabaseBrowserClient();
      const { error: updateErr } = await supabase.auth.updateUser({ email: newEmail });
      if (updateErr) {
        // Do NOT log the new email address — PII absence policy.
        console.warn("[change-email] updateUser failed:", updateErr.name);
        setError(classifyUpdateUserError(updateErr.message));
        return;
      }
      setSuccess(true);
    } finally {
      setLoading(false);
    }
  }

  if (success) {
    return (
      <div className="space-y-4">
        <p>
          Confirmation link sent to <strong>{newEmail}</strong>. Open the link in your email to
          finish the change.
        </p>
        <Button asChild>
          <Link href="/profile">Back to profile</Link>
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {error !== null ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {error}
        </div>
      ) : null}

      <div className="space-y-2">
        <Label>Current email</Label>
        {currentEmail !== null ? (
          <p>{currentEmail}</p>
        ) : (
          <p className="italic">No email on this account</p>
        )}
      </div>

      <div className="space-y-2">
        <Label htmlFor="new-email">New email</Label>
        <Input
          id="new-email"
          name="new-email"
          type="email"
          required
          value={newEmail}
          onChange={(e) => setNewEmail(e.target.value)}
        />
      </div>

      <div className="flex gap-2">
        <Button type="submit" disabled={loading}>
          {loading ? "Sending..." : "Send confirmation link"}
        </Button>
        <Button asChild variant="outline">
          <Link href="/profile">Cancel</Link>
        </Button>
      </div>
    </form>
  );
}

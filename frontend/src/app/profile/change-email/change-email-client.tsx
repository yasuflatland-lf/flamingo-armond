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

type Classification = { userMessage: string; classified: boolean };

function classifyUpdateUserError(message: string): Classification {
  const lower = message.toLowerCase();
  if (lower.includes("rate limit")) {
    return {
      userMessage: "Too many requests. Please wait a moment and try again.",
      classified: true,
    };
  }
  if (lower.includes("already registered")) {
    return { userMessage: "That email address is already in use.", classified: true };
  }
  // Generic copy for unmapped errors: avoids leaking technical strings or request IDs to the user.
  return { userMessage: "Could not send confirmation link. Please try again.", classified: false };
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
        const classification = classifyUpdateUserError(updateErr.message);
        if (classification.classified) {
          // Classified: operators know what happened from the user copy + error.name; no raw needed.
          console.warn("[change-email] updateUser failed:", updateErr.name);
        } else {
          // Unmapped: log the raw Supabase message so operators can extend classifyUpdateUserError.
          // Supabase API error messages are server-generated and do not echo user-typed input,
          // so this is PII-safe.
          console.warn(
            "[change-email] updateUser failed (unmapped):",
            updateErr.name,
            updateErr.message,
          );
        }
        setError(classification.userMessage);
        return;
      }
      setSuccess(true);
    } catch (err) {
      // Transport-level failure (network, timeout). The API-shaped failure goes via updateErr above.
      // Do NOT log err.message — Supabase exception messages can include the email the user typed.
      console.warn("[change-email] updateUser threw:", err instanceof Error ? err.name : "unknown");
      setError("Network error. Please check your connection and try again.");
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

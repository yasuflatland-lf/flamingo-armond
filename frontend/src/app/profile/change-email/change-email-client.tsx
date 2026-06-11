"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createSupabaseBrowserClient } from "@/lib/supabase/client";

type Props = {
  /**
   * The user's current email address. Required — callers must pass the value
   * or explicit null; never collapse to "". Per
   * `docs/frontend/typescript-conventions.md` § "Required `string |
   * null` over optional".
   */
  currentEmail: string | null;
};

// Returns the translation key for the mapped user-facing message, or null when
// the Supabase message is unmapped — null lets the caller log the raw message
// and show generic copy.
function classifyUpdateUserError(
  message: string,
): "emailRateLimited" | "emailAlreadyInUse" | null {
  const lower = message.toLowerCase();
  if (lower.includes("rate limit")) {
    return "emailRateLimited";
  }
  if (lower.includes("already registered")) {
    return "emailAlreadyInUse";
  }
  return null;
}

export function ChangeEmailClient({ currentEmail }: Props) {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");
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
        const classifiedKey = classifyUpdateUserError(updateErr.message);
        if (classifiedKey !== null) {
          // Classified: operators know what happened from the user copy + error.name; no raw needed.
          console.warn("[change-email] updateUser failed:", updateErr.name);
          setError(t(classifiedKey));
        } else {
          // Unmapped: log the raw Supabase message so operators can extend classifyUpdateUserError.
          // Supabase API error messages are server-generated and do not echo user-typed input,
          // so this is PII-safe.
          console.warn(
            "[change-email] updateUser failed (unmapped):",
            updateErr.name,
            updateErr.message,
          );
          setError(t("emailCouldNotSend"));
        }
        return;
      }
      setSuccess(true);
    } catch (err) {
      // Transport-level failure (network, timeout). The API-shaped failure goes via updateErr above.
      // Do NOT log err.message — Supabase exception messages can include the email the user typed.
      console.warn("[change-email] updateUser threw:", err instanceof Error ? err.name : "unknown");
      setError(t("emailNetworkError"));
    } finally {
      setLoading(false);
    }
  }

  if (success) {
    return (
      <div className="space-y-4">
        <p>
          {t.rich("confirmationSent", {
            email: newEmail,
            strong: (chunks) => <strong>{chunks}</strong>,
          })}
        </p>
        <Button asChild>
          <Link href="/profile">{t("backToProfile")}</Link>
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
        <Label>{t("currentEmail")}</Label>
        {currentEmail !== null ? (
          <p>{currentEmail}</p>
        ) : (
          <p className="italic">{t("noEmail")}</p>
        )}
      </div>

      <div className="space-y-2">
        <Label htmlFor="new-email">{t("newEmail")}</Label>
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
          {loading ? t("sending") : t("sendConfirmationLink")}
        </Button>
        <Button asChild variant="outline">
          <Link href="/profile">{tCommon("cancel")}</Link>
        </Button>
      </div>
    </form>
  );
}

"use client";

import { Pencil } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useRef, useState } from "react";
import { LanguageSwitcher } from "@/components/settings/language-switcher";
import { Button } from "@/components/ui/button";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import type { LearnDisplayMode } from "@/generated/graphql";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { DeleteAccountSection } from "./delete-account-section";
import { DisplayModeSection } from "./display-mode-section";
import { NewCardRatioSection } from "./new-card-ratio-section";
import { ProfileForm } from "./profile-form";

type Props = {
  email: string | null;
  initial: { displayName: string; bio: string };
  displayMode: LearnDisplayMode;
  // Structural shape rather than a query-derived type: the ratio is fetched by a
  // separate, admin-only, failure-tolerant query in page.tsx and falls back to a
  // plain default object, so it is not tied to any single query's result type.
  newCardRatio: { numerator: number; denominator: number };
  isAdmin: boolean;
};

function ProfileSheetBody({
  email,
  initial,
  onChangeEmail,
  onDirtyChange,
  onRegisterReset,
  onSaved,
  onSubmittingChange,
}: Pick<Props, "email" | "initial"> & {
  onChangeEmail: () => void;
  onDirtyChange: (dirty: boolean) => void;
  onRegisterReset: (reset: () => void) => void;
  onSaved: () => void;
  onSubmittingChange: (submitting: boolean) => void;
}) {
  const close = useFormSheetClose();

  return (
    <ProfileForm
      email={email}
      initial={initial}
      onCancel={close}
      onChangeEmail={onChangeEmail}
      onDirtyChange={onDirtyChange}
      onRegisterReset={onRegisterReset}
      onSaved={onSaved}
      onSubmittingChange={onSubmittingChange}
    />
  );
}

// `/profile` edits a singleton aggregate (the signed-in user's own profile)
// that has no per-entity id in the URL space. `useSheetSearchParam` expects an
// `id` for `mode: "edit"`, so this page uses a fixed sentinel that cannot
// collide with any real entity id (no other admin surface routes "self" to an
// entity). The contract is local to this page; the hook stays unaware.
const PROFILE_SHEET_SENTINEL_ID = "self";

export function ProfilePageClient({ email, initial, displayMode, newCardRatio, isAdmin }: Props) {
  const t = useTranslations("Profile");
  const tSettings = useTranslations("Settings");
  const router = useRouter();
  const sheet = useSheetSearchParam();
  const [dirty, setDirty] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const resetProfileFormRef = useRef<() => void>(() => {});
  const open = sheet.state.mode === "edit" && sheet.state.id === PROFILE_SHEET_SENTINEL_ID;

  const resetBeforeClose = useCallback(() => {
    resetProfileFormRef.current();
    setDirty(false);
    setSubmitting(false);
  }, []);

  const handleSheetOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (nextOpen) {
        sheet.open({ mode: "edit", id: PROFILE_SHEET_SENTINEL_ID });
        return;
      }

      resetBeforeClose();
      sheet.close();
    },
    [resetBeforeClose, sheet],
  );

  const handleSaved = useCallback(() => {
    resetBeforeClose();
    sheet.close({ refresh: true });
  }, [resetBeforeClose, sheet]);

  const handleChangeEmail = useCallback(() => {
    if (submitting) {
      return;
    }

    resetBeforeClose();
    sheet.close();
    router.push("/profile/change-email");
  }, [resetBeforeClose, router, sheet, submitting]);

  return (
    <main className="p-8">
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-8">
        <section className="space-y-3 border-b pb-6">
          <div>
            <div className="flex items-center gap-1">
              <h1 className="text-2xl font-semibold">{t("title")}</h1>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8 text-muted-foreground"
                aria-label={t("editProfile")}
                onClick={() => sheet.open({ mode: "edit", id: PROFILE_SHEET_SENTINEL_ID })}
              >
                <Pencil className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <dl className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-1">
              <dt className="text-sm font-medium text-muted-foreground">{t("displayName")}</dt>
              <dd className="break-words text-sm">{initial.displayName || t("notSet")}</dd>
            </div>
            <div className="space-y-1">
              <dt className="text-sm font-medium text-muted-foreground">{t("email")}</dt>
              <dd className="break-words text-sm">
                {email ?? <span className="italic">{t("noEmail")}</span>}
              </dd>
            </div>
            <div className="space-y-1">
              <dt className="text-sm font-medium text-muted-foreground">{t("bio")}</dt>
              <dd className="break-words text-sm">{initial.bio || t("notSet")}</dd>
            </div>
          </dl>
        </section>

        <section className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">{tSettings("heading")}</h2>
          </div>
          <LanguageSwitcher />
          <DisplayModeSection initialMode={displayMode} />
          {isAdmin ? <NewCardRatioSection initialRatio={newCardRatio} /> : null}
        </section>

        <DeleteAccountSection />
      </div>

      <FormSheet
        open={open}
        onOpenChange={handleSheetOpenChange}
        title={t("editProfile")}
        dirty={dirty}
        submitting={submitting}
        size="md"
      >
        <ProfileSheetBody
          key={open ? "open" : "closed"}
          email={email}
          initial={initial}
          onChangeEmail={handleChangeEmail}
          onDirtyChange={setDirty}
          onRegisterReset={(reset) => {
            resetProfileFormRef.current = reset;
          }}
          onSaved={handleSaved}
          onSubmittingChange={setSubmitting}
        />
      </FormSheet>
    </main>
  );
}

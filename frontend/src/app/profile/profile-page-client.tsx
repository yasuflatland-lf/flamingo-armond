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
import { ProfileForm } from "./profile-form";

type Props = {
  email: string | null;
  initial: { displayName: string; bio: string };
  displayMode: LearnDisplayMode;
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

export function ProfilePageClient({ email, initial, displayMode }: Props) {
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
        <section className="flex flex-wrap items-start gap-4 border-b pb-6">
          <div className="min-w-0 flex-1 space-y-3">
            <div>
              <h1 className="text-2xl font-semibold">{t("title")}</h1>
              <p className="text-sm text-muted-foreground">{t("readOnlySummary")}</p>
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
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => sheet.open({ mode: "edit", id: PROFILE_SHEET_SENTINEL_ID })}
          >
            <Pencil className="h-4 w-4" />
            {t("editProfile")}
          </Button>
        </section>

        <section className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">{tSettings("heading")}</h2>
            <p className="text-sm text-muted-foreground">{tSettings("description")}</p>
          </div>
          <LanguageSwitcher />
          <DisplayModeSection initialMode={displayMode} />
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

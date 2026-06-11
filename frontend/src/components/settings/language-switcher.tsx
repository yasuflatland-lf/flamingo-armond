"use client";

import { useLocale, useTranslations } from "next-intl";
import { useId, useTransition } from "react";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { locales, toLocale } from "@/i18n/config";
import { setUserLocale } from "@/i18n/locale-actions";

/**
 * Language picker backed by the `NEXT_LOCALE` cookie. Selecting a locale calls
 * the `setUserLocale` Server Action inside a transition; `next-intl` re-resolves
 * the active locale from the cookie on the next render. The control is disabled
 * while the transition is pending so a second choice cannot race the first.
 */
export function LanguageSwitcher() {
  const t = useTranslations("Language");
  const currentLocale = useLocale();
  const [isPending, startTransition] = useTransition();
  const triggerId = useId();

  function handleValueChange(value: string) {
    const next = toLocale(value);
    if (next === null) {
      return;
    }
    startTransition(() => {
      setUserLocale(next);
    });
  }

  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={triggerId}>{t("label")}</Label>
      <Select value={currentLocale} onValueChange={handleValueChange} disabled={isPending}>
        <SelectTrigger id={triggerId} className="w-full sm:w-56">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {locales.map((locale) => (
            <SelectItem key={locale} value={locale}>
              {t(locale)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

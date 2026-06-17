"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MasterCardsSection } from "./master-cards-section";
import { MasterEditHeader } from "./master-edit-header";
import type { AdminMasterDeck } from "./queries";

type Props = { master: AdminMasterDeck };

export function MasterManagementClient({ master }: Props) {
  const t = useTranslations("AdminMasters");
  return (
    <main className="p-4 md:p-8">
      <div className="mb-2">
        <Link
          href="/admin/masters"
          className="inline-flex text-sm text-muted-foreground hover:text-foreground hover:underline"
        >
          {t("backToList")}
        </Link>
      </div>

      <MasterEditHeader master={master} />
      <MasterCardsSection masterId={master.id} />
    </main>
  );
}

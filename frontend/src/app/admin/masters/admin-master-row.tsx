"use client";

import { useMutation } from "@apollo/client/react";
import { Pencil } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { AdminPublishMasterMutation, AdminUnpublishMasterMutation } from "./queries";

type MasterStatus = "DRAFT" | "PUBLISHED";

/**
 * Display shape for an admin master row + edit form. Nullable string fields are
 * `string | null` (required, nullable), not optional — see
 * docs/frontend/typescript-conventions/required-string-null-over-optional-string-null.md.
 */
export type AdminMasterListItem = {
  id: string;
  version: number;
  name: string;
  description: string | null;
  language: string | null;
  level: string | null;
  category: string | null;
  coverImageUrl: string | null;
  source: string | null;
  isDefaultStarter: boolean;
  sortOrder: number;
  status: MasterStatus;
  cardCount: number;
};

type Props = {
  master: AdminMasterListItem;
  onEdit: (id: string) => void;
};

export function AdminMasterRow({ master, onEdit }: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");

  const published = master.status === "PUBLISHED";
  const emptyDraft = master.status === "DRAFT" && master.cardCount === 0;

  const [runPublish, { loading: publishing }] = useMutation(AdminPublishMasterMutation);
  const [runUnpublish, { loading: unpublishing }] = useMutation(AdminUnpublishMasterMutation);
  const busy = publishing || unpublishing;

  async function handleToggle() {
    // Empty-deck Publish cannot proceed; surface the reason via the shared toast
    // notification instead of a persistent inline hint.
    if (emptyDraft) {
      toast.error(t("publishEmptyHint"));
      return;
    }
    try {
      if (published) {
        const result = await runUnpublish({ variables: { id: master.id } });
        if (!result.data?.adminUnpublishMasterCardgroup) {
          console.warn("[admin-masters] unpublish returned null payload", { masterId: master.id });
          toast.error(t("unexpectedError"));
          return;
        }
        toast.success(t("unpublishSuccess"));
        return;
      }
      const result = await runPublish({ variables: { id: master.id } });
      const payload = result.data?.adminPublishMasterCardgroup;
      // Capture the typename before narrowing exhausts the union type below.
      const typename = payload?.__typename ?? "null";
      if (payload?.__typename === "PublishMasterCardgroupSuccess") {
        toast.success(t("publishSuccess"));
        return;
      }
      if (payload?.__typename === "MasterCardgroupEmptyError") {
        toast.error(t("publishEmptyToast"));
        return;
      }
      // Unexpected payload shape (null or unknown variant) — never fail silently.
      console.warn("[admin-masters] publish returned unexpected payload", {
        masterId: master.id,
        typename,
      });
      toast.error(t("unexpectedError"));
    } catch (err) {
      // err.message omitted — backend messages may carry content. See
      // docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
      const codes = liftGraphQLCodes(err);
      console.warn("[admin-masters] publish toggle failed", {
        masterId: master.id,
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      toast.error(
        codes.includes("FORBIDDEN")
          ? t("forbidden")
          : codes.includes("UNAUTHENTICATED")
            ? t("unauthenticated")
            : t("unexpectedError"),
      );
    }
  }

  const toggleLabel = published ? t("unpublish") : t("publish");

  return (
    <li
      className="rounded-md border border-border transition-colors hover:bg-accent"
      data-testid={`master-catalog-row-${master.id}`}
    >
      <div className="flex flex-wrap items-center gap-3 px-4 py-3">
        <Badge
          variant={published ? "default" : "secondary"}
          role="status"
          data-testid="master-row-status-badge"
        >
          {published ? t("statusPublished") : t("statusDraft")}
        </Badge>

        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium" title={master.name}>
            {master.name}
          </p>
        </div>

        <span className="shrink-0 text-xs text-muted-foreground">
          {t("cardCount", { count: master.cardCount })}
        </span>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0"
          data-testid="master-row-publish-toggle"
          aria-pressed={published}
          // Empty drafts stay clickable (no aria-disabled): a click surfaces the
          // "add cards first" guidance via the shared toast (see handleToggle)
          // rather than a persistent inline hint.
          onClick={busy ? undefined : handleToggle}
        >
          {busy ? tCommon("loading") : toggleLabel}
        </Button>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0"
          data-testid="master-row-edit"
          onClick={() => onEdit(master.id)}
          aria-label={t("editAriaLabel", { name: master.name })}
        >
          <Pencil aria-hidden="true" />
          <span>{tCommon("edit")}</span>
        </Button>
      </div>
    </li>
  );
}

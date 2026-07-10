"use client";

import { Plus } from "lucide-react";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";
import { RoleForm } from "@/components/admin/role-form";
import { RoleListItem } from "@/components/admin/role-list-item";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { AuthErrorBanner } from "@/components/ui/auth-error-banner";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { useUndoDelete } from "@/lib/undo-delete";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { SYSTEM_ROLE_NAMES } from "./queries";
import { useRoleMutations } from "./use-role-mutations";

/** Auth banner discriminant carried by the collapsed create/edit error state. */
type AuthKind = "unauthenticated" | "forbidden";

/** Collapsed create-sheet banner state — one of the typed outcomes the hook returns. */
type CreateError =
  | { kind: "validation"; field: string; message: string }
  | { kind: "auth"; authError: AuthKind }
  | { kind: "unexpected" };

/** Collapsed edit-sheet banner state — one of the typed outcomes the hook returns. */
type EditError =
  | { kind: "systemRole"; message: string }
  | { kind: "auth"; authError: AuthKind }
  | { kind: "unexpected" };

export type RoleItem = { id: string; name: string };

type Props = { initialRoles: RoleItem[] };

type ValidationError = { field: string; message: string };

function CreateRoleSheetBody({
  submitting,
  validationError,
  authError,
  unexpectedPayloadError,
  onDirty,
  onDirtyChange,
  submit,
}: {
  submitting: boolean;
  validationError: ValidationError | null;
  authError: "unauthenticated" | "forbidden" | null;
  unexpectedPayloadError: string | null;
  onDirty: () => void;
  onDirtyChange: (dirty: boolean) => void;
  submit: (values: { name: string }) => Promise<void>;
}) {
  const t = useTranslations("Admin");
  const close = useFormSheetClose();

  return (
    <div onInput={onDirty}>
      {authError ? (
        <AuthErrorBanner
          testId="admin-role-new-auth-error"
          message={authError === "unauthenticated" ? t("sessionExpired") : t("forbidden")}
          signInLabel={t("signInAgain")}
        />
      ) : null}
      {validationError ? (
        <ErrorBanner data-testid="admin-role-new-validation-error">
          {validationError.message}
        </ErrorBanner>
      ) : null}
      {unexpectedPayloadError ? (
        <ErrorBanner data-testid="admin-role-new-unexpected-payload-error">
          {unexpectedPayloadError}
        </ErrorBanner>
      ) : null}

      <RoleForm
        defaultValues={{ name: "" }}
        submit={submit}
        submitLabel={t("createRole")}
        submitting={submitting}
        onCancel={close}
        onDirtyChange={onDirtyChange}
      />
    </div>
  );
}

function EditRoleSheetBody({
  role,
  loading,
  submitting,
  mutationError,
  authError,
  systemRoleError,
  unexpectedPayloadError,
  queryErrorBanner,
  submit,
}: {
  role: RoleItem | null;
  loading: boolean;
  submitting: boolean;
  mutationError: unknown;
  authError: "unauthenticated" | "forbidden" | null;
  systemRoleError: string | null;
  unexpectedPayloadError: string | null;
  queryErrorBanner: string | undefined;
  submit: (values: { name: string }) => Promise<void>;
}) {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const close = useFormSheetClose();

  if (!role) {
    return (
      <div className="space-y-4">
        {queryErrorBanner ? (
          <ErrorBanner>{queryErrorBanner}</ErrorBanner>
        ) : loading ? (
          <p className="text-sm text-muted-foreground">{t("loadingRole")}</p>
        ) : (
          <ErrorBanner data-testid="admin-role-edit-not-found">{t("roleNotFound")}</ErrorBanner>
        )}
      </div>
    );
  }

  const readOnly = SYSTEM_ROLE_NAMES.has(role.name);

  return (
    <div className="space-y-4">
      {queryErrorBanner ? <ErrorBanner>{queryErrorBanner}</ErrorBanner> : null}
      {readOnly ? (
        <div
          role="status"
          data-testid="admin-role-edit-system-banner"
          className="rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
        >
          {t("systemRoleBanner")}
        </div>
      ) : null}
      {authError ? (
        <AuthErrorBanner
          testId="admin-role-edit-auth-error"
          message={authError === "unauthenticated" ? t("sessionExpired") : t("forbidden")}
          signInLabel={t("signInAgain")}
        />
      ) : null}
      {systemRoleError ? (
        <ErrorBanner data-testid="admin-role-edit-system-role-error">{systemRoleError}</ErrorBanner>
      ) : null}
      {unexpectedPayloadError ? (
        <ErrorBanner data-testid="admin-role-edit-unexpected-payload-error">
          {unexpectedPayloadError}
        </ErrorBanner>
      ) : null}

      <RoleForm
        defaultValues={{ name: role.name }}
        submit={submit}
        submitLabel={tCommon("save")}
        submitting={submitting}
        error={mutationError}
        readOnly={readOnly}
        onCancel={close}
      />
    </div>
  );
}

/** Convert any thrown value to a user-facing string. */
function toMessage(err: unknown): string {
  return getBackendErrorBanner(err) ?? (err instanceof Error ? err.message : String(err));
}

export function AdminRolesClient({ initialRoles }: Props) {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const tNav = useTranslations("Nav");
  const [roles, setRoles] = useState<RoleItem[]>(initialRoles);
  const [createDirty, setCreateDirty] = useState(false);
  const [createError, setCreateError] = useState<CreateError | null>(null);
  const [editError, setEditError] = useState<EditError | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const {
    createRole,
    updateRole,
    deleteRole,
    loadRole,
    editRoleData,
    loadingEditRole,
    editRoleQueryError,
    loadRoleCalled,
    loadRoleVariables,
    updateMutationError,
    creating,
    updating,
    deleting,
    resetCreateRole,
    resetUpdateRole,
  } = useRoleMutations();
  const { scheduleDelete } = useUndoDelete();
  const { state, open, close } = useSheetSearchParam();
  const sheetMode = state.mode;
  const editId = state.mode === "edit" ? state.id : null;

  const resetCreateSheetState = useCallback(() => {
    setCreateError(null);
    setCreateDirty(false);
    resetCreateRole();
  }, [resetCreateRole]);

  const resetEditSheetState = useCallback(() => {
    setEditError(null);
    resetUpdateRole();
  }, [resetUpdateRole]);

  useEffect(() => {
    setRoles(initialRoles);
  }, [initialRoles]);

  useEffect(() => {
    if (!editId) {
      return;
    }
    void loadRole({ variables: { id: editId } }).catch((err) => {
      const codes = liftGraphQLCodes(err);
      console.warn("[admin/roles] loadRole rejected", {
        roleId: editId,
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
    });
  }, [editId, loadRole]);

  // Reset the create-sheet state on every sheet transition so a stale
  // validation banner from a previous attempt does not leak into the next open.
  // biome-ignore lint/correctness/useExhaustiveDependencies: `sheetMode` is a trigger-only dependency; the effect resets derived state and does not reference it in its body.
  useEffect(() => {
    resetCreateSheetState();
  }, [resetCreateSheetState, sheetMode]);

  // Reset the edit-sheet state on every sheet transition. `editId` and
  // `sheetMode` are intentional trigger dependencies: re-opening the edit sheet
  // with a different id (or switching modes) is a fresh attempt that must clear
  // any stale banner from a previous attempt.
  // biome-ignore lint/correctness/useExhaustiveDependencies: `editId` and `sheetMode` are trigger-only dependencies; the effect resets derived state and does not reference them in its body.
  useEffect(() => {
    resetEditSheetState();
  }, [editId, resetEditSheetState, sheetMode]);

  const loadedEditRole = editRoleData?.role as unknown as RoleItem | undefined;
  const editRole = state.mode === "edit" && loadedEditRole?.id === state.id ? loadedEditRole : null;
  const editQueryErrorBanner = getBackendErrorBanner(editRoleQueryError);
  // The lazy query is fired inside a `useEffect`, so on the first render
  // after `?edit=<id>` lands, `loadingEditRole` is still false. Treat the
  // "called for the current id" gap as loading so the body does not flash
  // its `Role not found` branch before the query is in flight.
  // Mirror of admin-users-client.tsx's `editUserCalled` / `editUserResultMatchesSheet` gate.
  const loadRoleMatchesSheet = editId !== null && loadRoleVariables?.id === editId;
  const editRoleLoading =
    loadingEditRole || (editId !== null && (!loadRoleCalled || !loadRoleMatchesSheet));

  function handleDelete(id: string) {
    const index = roles.findIndex((r) => r.id === id);
    const role = roles[index];
    if (!role) return;

    // Clear any stale error banner so a new attempt starts clean.
    setDeleteError(null);
    setRoles((prev) => prev.filter((r) => r.id !== id));

    scheduleDelete({
      id,
      label: `Role "${role.name}" deleted`,
      optimisticRollback: () => {
        setRoles((prev) => [...prev.slice(0, index), role, ...prev.slice(index)]);
      },
      commitDelete: () => deleteRole(id),
      onCommitFailed: (err) => {
        const codes = liftGraphQLCodes(err);
        // This branch deliberately does NOT route through the shared
        // classifyAndLogAuthOutcome helper (used by useRoleMutations for the
        // create/update branches): that is kind-only and would discard the
        // server's FORBIDDEN message. UNAUTHENTICATED collapses to a generic
        // sign-in prompt — the user has no actionable detail to recover from.
        // FORBIDDEN keeps the server's specific reason because the same code
        // covers system-role protection, where the backend message
        // (`cannot delete system role "admin"`) is what the operator needs to
        // see. deleteRole returns Boolean! (no typed outcome union), so this
        // FORBIDDEN passthrough is the only channel for that message — unlike
        // update, whose system-role guard travels via CannotModifySystemRoleError.
        if (codes.includes("UNAUTHENTICATED")) {
          setDeleteError(t("deleteAuthFailed"));
          console.warn("[admin/roles] deleteRole auth failure", { roleId: id, codes });
          return;
        }
        console.warn("[admin/roles] deleteRole rejected", {
          roleId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        setDeleteError(toMessage(err));
      },
    });
  }

  async function handleCreateSubmit(values: { name: string }) {
    setCreateError(null);

    const outcome = await createRole(values);
    switch (outcome.status) {
      case "success":
        resetCreateSheetState();
        close({ refresh: true });
        return;
      case "validation":
        setCreateError({ kind: "validation", field: outcome.field, message: outcome.message });
        return;
      case "auth":
        setCreateError({ kind: "auth", authError: outcome.kind });
        return;
      case "unexpected":
        setCreateError({ kind: "unexpected" });
        return;
      default:
        // "rejected" — classifyAndLogAuthOutcome already emitted a scoped warn.
        return;
    }
  }

  async function handleEditSubmit(values: { name: string }) {
    if (!editRole) return;

    setEditError(null);

    const outcome = await updateRole(editRole.id, values);
    switch (outcome.status) {
      case "success":
        resetEditSheetState();
        close({ refresh: true });
        return;
      case "systemRole":
        setEditError({ kind: "systemRole", message: outcome.message });
        return;
      case "auth":
        setEditError({ kind: "auth", authError: outcome.kind });
        return;
      case "unexpected":
        setEditError({ kind: "unexpected" });
        return;
      default:
        // "rejected" — classifyAndLogAuthOutcome already emitted a scoped warn.
        return;
    }
  }

  return (
    <ListingPageShell
      title={tNav("roles")}
      count={roles.length}
      countLabel={tCommon("totalCount", { count: roles.length })}
      primaryActions={
        <Button
          type="button"
          variant="brand"
          className="hidden md:inline-flex"
          data-testid="admin-roles-new-btn"
          onClick={() => open({ mode: "new" })}
        >
          <span>{t("newRole")}</span>
          <Plus aria-hidden="true" />
        </Button>
      }
    >
      {roles.length === 0 && !deleteError ? (
        <div
          className="flex flex-col items-center gap-3 py-8 text-center"
          data-testid="admin-roles-empty"
        >
          <p className="text-sm text-muted-foreground">{t("noRolesYet")}</p>
          <Button
            type="button"
            variant="brand"
            onClick={() => open({ mode: "new" })}
            data-testid="admin-roles-empty-cta"
          >
            <span>{t("newRole")}</span>
            <Plus aria-hidden="true" />
          </Button>
        </div>
      ) : (
        <ul className="space-y-3" data-testid="admin-roles-list">
          {deleteError ? (
            <li>
              <ErrorBanner data-testid="admin-roles-error">{deleteError}</ErrorBanner>
            </li>
          ) : null}
          {roles.map((role) => (
            <RoleListItem
              key={role.id}
              id={role.id}
              name={role.name}
              isSystem={SYSTEM_ROLE_NAMES.has(role.name)}
              busy={deleting}
              onEdit={(roleId) => open({ mode: "edit", id: roleId })}
              onDelete={handleDelete}
            />
          ))}
        </ul>
      )}

      <FormSheet
        title={t("newRole")}
        open={state.mode === "new"}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            resetCreateSheetState();
            close();
          }
        }}
        submitting={creating}
        dirty={createDirty}
        confirmOnDismiss
        size="sm"
      >
        {state.mode === "new" ? (
          <CreateRoleSheetBody
            submitting={creating}
            validationError={createError?.kind === "validation" ? createError : null}
            authError={createError?.kind === "auth" ? createError.authError : null}
            unexpectedPayloadError={
              createError?.kind === "unexpected" ? tCommon("somethingWentWrong") : null
            }
            onDirty={() => setCreateError((prev) => (prev?.kind === "auth" ? prev : null))}
            onDirtyChange={setCreateDirty}
            submit={handleCreateSubmit}
          />
        ) : null}
      </FormSheet>

      <FormSheet
        title={t("editRoleTitle")}
        open={state.mode === "edit"}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            resetEditSheetState();
            close();
          }
        }}
        submitting={updating || loadingEditRole}
        confirmOnDismiss={false}
        size="sm"
      >
        {state.mode === "edit" ? (
          <EditRoleSheetBody
            role={editRole}
            loading={editRoleLoading}
            submitting={updating}
            mutationError={updateMutationError}
            authError={editError?.kind === "auth" ? editError.authError : null}
            systemRoleError={editError?.kind === "systemRole" ? editError.message : null}
            unexpectedPayloadError={
              editError?.kind === "unexpected" ? tCommon("somethingWentWrong") : null
            }
            queryErrorBanner={editQueryErrorBanner}
            submit={handleEditSubmit}
          />
        ) : null}
      </FormSheet>
    </ListingPageShell>
  );
}

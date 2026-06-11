"use client";

import { useLazyQuery, useMutation } from "@apollo/client/react";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";
import { RoleForm } from "@/components/admin/role-form";
import { RoleListItem } from "@/components/admin/role-list-item";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { useUndoDelete } from "@/lib/undo-delete";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import {
  AdminCreateRoleMutation,
  AdminDeleteRoleMutation,
  AdminRoleQuery,
  AdminUpdateRoleMutation,
  SYSTEM_ROLE_NAMES,
} from "./queries";

export type RoleItem = { id: string; name: string };

type Props = { initialRoles: RoleItem[] };

type ValidationError = { field: string; message: string };

function AuthBanner({
  testId,
  authError,
}: {
  testId: string;
  authError: "unauthenticated" | "forbidden";
}) {
  const t = useTranslations("Admin");
  return (
    <div
      role="alert"
      data-testid={testId}
      className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
    >
      <span>{authError === "unauthenticated" ? t("sessionExpired") : t("forbidden")}</span>{" "}
      <Link href="/login" className="underline">
        {t("signInAgain")}
      </Link>
      .
    </div>
  );
}

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
      {authError ? <AuthBanner testId="admin-role-new-auth-error" authError={authError} /> : null}
      {validationError ? (
        <div
          role="alert"
          data-testid="admin-role-new-validation-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {validationError.message}
        </div>
      ) : null}
      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="admin-role-new-unexpected-payload-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
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
          <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {queryErrorBanner}
          </div>
        ) : loading ? (
          <p className="text-sm text-muted-foreground">{t("loadingRole")}</p>
        ) : (
          <div
            role="alert"
            data-testid="admin-role-edit-not-found"
            className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          >
            {t("roleNotFound")}
          </div>
        )}
      </div>
    );
  }

  const readOnly = SYSTEM_ROLE_NAMES.has(role.name);

  return (
    <div className="space-y-4">
      {queryErrorBanner ? (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {queryErrorBanner}
        </div>
      ) : null}
      {readOnly ? (
        <div
          role="status"
          data-testid="admin-role-edit-system-banner"
          className="rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
        >
          {t("systemRoleBanner")}
        </div>
      ) : null}
      {authError ? <AuthBanner testId="admin-role-edit-auth-error" authError={authError} /> : null}
      {systemRoleError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-system-role-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {systemRoleError}
        </div>
      ) : null}
      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-unexpected-payload-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
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
  const [createValidationError, setCreateValidationError] = useState<ValidationError | null>(null);
  const [createUnexpectedPayloadError, setCreateUnexpectedPayloadError] = useState<string | null>(
    null,
  );
  const [createAuthError, setCreateAuthError] = useState<"unauthenticated" | "forbidden" | null>(
    null,
  );
  const [editSystemRoleError, setEditSystemRoleError] = useState<string | null>(null);
  const [editUnexpectedPayloadError, setEditUnexpectedPayloadError] = useState<string | null>(null);
  const [editAuthError, setEditAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const [createRole, { loading: creating, reset: resetCreateRole }] =
    useMutation(AdminCreateRoleMutation);
  const [updateRole, { loading: updating, error: updateMutationError, reset: resetUpdateRole }] =
    useMutation(AdminUpdateRoleMutation);
  const [deleteRoleMutate, { loading: deleting }] = useMutation(AdminDeleteRoleMutation);
  const [
    loadRole,
    {
      data: editRoleData,
      loading: loadingEditRole,
      error: editRoleQueryError,
      called: loadRoleCalled,
      variables: loadRoleVariables,
    },
  ] = useLazyQuery(AdminRoleQuery, { fetchPolicy: "no-cache" });
  const { scheduleDelete } = useUndoDelete();
  const { state, open, close } = useSheetSearchParam();
  const sheetMode = state.mode;
  const editId = state.mode === "edit" ? state.id : null;

  const resetCreateSheetState = useCallback(() => {
    setCreateValidationError(null);
    setCreateUnexpectedPayloadError(null);
    setCreateAuthError(null);
    setCreateDirty(false);
    resetCreateRole();
  }, [resetCreateRole]);

  const resetEditSheetState = useCallback(() => {
    setEditSystemRoleError(null);
    setEditUnexpectedPayloadError(null);
    setEditAuthError(null);
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
      commitDelete: () => deleteRoleMutate({ variables: { id } }),
      onCommitFailed: (err) => {
        const codes = liftGraphQLCodes(err);
        // UNAUTHENTICATED collapses to a generic sign-in prompt — the user has
        // no actionable detail to recover from. FORBIDDEN keeps the server's
        // specific reason because the same code covers system-role races where
        // the backend message ("cannot delete a protected role") is what the
        // operator needs to see.
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
    setCreateValidationError(null);
    setCreateUnexpectedPayloadError(null);
    setCreateAuthError(null);

    const name = values.name.trim().toLowerCase();
    const result = await createRole({ variables: { name } }).catch((err) => {
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setCreateAuthError("unauthenticated");
        return null;
      }
      if (codes.includes("FORBIDDEN")) {
        setCreateAuthError("forbidden");
        return null;
      }
      // err.message is omitted — backend messages may echo user input.
      // codes is safe to log (fixed enum of GraphQL extension codes).
      console.warn("[admin/roles] createRole rejected", {
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });

    if (!result) return;

    const payload = result.data?.createRole;
    const typename = payload?.__typename ?? null;

    if (payload?.__typename === "InputValidationError") {
      setCreateValidationError({ field: payload.field, message: payload.message });
      return;
    }

    if (payload?.__typename === "CreateRoleSuccess") {
      resetCreateSheetState();
      close({ refresh: true });
      return;
    }

    console.warn("[admin/roles] unexpected createRole payload", {
      typename,
    });
    setCreateUnexpectedPayloadError(tCommon("somethingWentWrong"));
  }

  async function handleEditSubmit(values: { name: string }) {
    if (!editRole) return;

    setEditSystemRoleError(null);
    setEditUnexpectedPayloadError(null);
    setEditAuthError(null);

    const name = values.name.trim().toLowerCase();
    const result = await updateRole({ variables: { id: editRole.id, name } }).catch((err) => {
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setEditAuthError("unauthenticated");
        return null;
      }
      if (codes.includes("FORBIDDEN")) {
        setEditAuthError("forbidden");
        return null;
      }
      console.warn("[admin/roles] updateRole rejected", {
        roleId: editRole.id,
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });

    if (!result) return;

    const payload = result.data?.updateRole;
    const typename = payload?.__typename ?? null;

    if (payload?.__typename === "CannotModifySystemRoleError") {
      setEditSystemRoleError(payload.message);
      return;
    }

    if (payload?.__typename === "UpdateRoleSuccess") {
      resetEditSheetState();
      close({ refresh: true });
      return;
    }

    console.warn("[admin/roles] unexpected updateRole payload", {
      typename,
    });
    setEditUnexpectedPayloadError(tCommon("somethingWentWrong"));
  }

  return (
    <ListingPageShell
      title={tNav("roles")}
      description={t("rolesDescription")}
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
              <div
                role="alert"
                data-testid="admin-roles-error"
                className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
              >
                {deleteError}
              </div>
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
            validationError={createValidationError}
            authError={createAuthError}
            unexpectedPayloadError={createUnexpectedPayloadError}
            onDirty={() => {
              setCreateValidationError(null);
              setCreateUnexpectedPayloadError(null);
            }}
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
            authError={editAuthError}
            systemRoleError={editSystemRoleError}
            unexpectedPayloadError={editUnexpectedPayloadError}
            queryErrorBanner={editQueryErrorBanner}
            submit={handleEditSubmit}
          />
        ) : null}
      </FormSheet>
    </ListingPageShell>
  );
}

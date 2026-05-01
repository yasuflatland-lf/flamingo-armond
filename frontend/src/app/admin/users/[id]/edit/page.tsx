import { redirect } from "next/navigation";
import type {
  AdminRolesQuery as AdminRolesQueryType,
  AdminUserQuery as AdminUserQueryType,
} from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { AdminRolesQuery, AdminUserQuery } from "../../queries";
import { AdminUserEditClient, type RoleOption, type UserForEdit } from "./AdminUserEditClient";

export default async function AdminUserEditPage({ params }: { params: Promise<{ id: string }> }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr) throw authErr;
  if (!user) redirect("/login");

  const { id } = await params;

  let userData: AdminUserQueryType | null = null;
  let rolesData: AdminRolesQueryType | null = null;

  try {
    [userData, rolesData] = await Promise.all([
      gqlFetch(AdminUserQuery, { variables: { id }, revalidate: 0 }),
      gqlFetch(AdminRolesQuery, { revalidate: 0 }),
    ]);
  } catch (err) {
    redirectIfUnauthenticated(err, "/");
  }

  if (!userData?.adminUser) redirect("/admin/users");

  // The fragment-masked types carry the actual field values at runtime. Cast to
  // the plain serializable props the client component expects.
  const userForEdit = userData.adminUser as unknown as UserForEdit;
  const allRoles = (rolesData?.roles ?? []) as unknown as RoleOption[];

  return <AdminUserEditClient user={userForEdit} allRoles={allRoles} />;
}

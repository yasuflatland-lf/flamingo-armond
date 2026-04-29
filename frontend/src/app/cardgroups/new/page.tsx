import { redirect } from "next/navigation";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { NewCardgroupClient } from "./new-cardgroup-client";

export default async function NewCardgroupPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr) throw authErr;
  if (!user) redirect("/login");

  return <NewCardgroupClient />;
}

import { redirect } from "next/navigation";
import { graphql } from "@/generated";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { ProfileForm } from "./profile-form";

const MeQuery = graphql(`
  query Me {
    me {
      id
      displayName
      bio
      avatarUrl
    }
  }
`);

export default async function ProfilePage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[profile] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  const data = await gqlFetch(MeQuery, { revalidate: 0 });
  if (!data.me) {
    throw new Error("/profile: me returned null with no error");
  }

  return (
    <main className="mx-auto max-w-xl p-8">
      <h1 className="mb-6 text-2xl font-semibold">Edit profile</h1>
      <ProfileForm
        initial={{
          displayName: data.me.displayName ?? "",
          bio: data.me.bio ?? "",
        }}
      />
    </main>
  );
}

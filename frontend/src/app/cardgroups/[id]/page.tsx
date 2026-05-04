import { redirect } from "next/navigation";

// /cardgroups/[id] is preserved (not deleted) so existing bookmarks and
// third-party links keep working. Auth is intentionally not re-checked here
// because the destination page does its own auth check.
export default async function CardgroupDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  redirect(`/cardgroups/${encodeURIComponent(id)}/cards`);
}

import { redirect } from "next/navigation";

type Props = { params: Promise<{ id: string }> };

export default async function CardgroupDetailPage({ params }: Props) {
  const { id } = await params;
  redirect(`/cardgroups/${id}/edit`);
}

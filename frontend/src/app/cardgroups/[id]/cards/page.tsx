import { redirect } from "next/navigation";

type Props = { params: Promise<{ id: string }> };

export default async function CardsPage({ params }: Props) {
  const { id } = await params;
  redirect(`/cardgroups/${id}/edit`);
}

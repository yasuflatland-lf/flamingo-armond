import type { Metadata } from "next";
import { redirect } from "next/navigation";

// Per-user private route — must not be indexed.
export const metadata: Metadata = { robots: { index: false, follow: false } };

type Props = { params: Promise<{ id: string }> };

export default async function CardsPage({ params }: Props) {
  const { id } = await params;
  redirect(`/cardgroups/${id}/edit`);
}

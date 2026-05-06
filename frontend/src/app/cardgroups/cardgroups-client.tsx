"use client";

import type { MyCardgroupsConnectionQuery } from "@/generated/graphql";

type CardgroupConnection = MyCardgroupsConnectionQuery["myCardgroupsConnection"];

interface CardgroupsClientProps {
  initialConnection: CardgroupConnection | null;
}

/**
 * Stub for the /cardgroups client component.
 * The full implementation (search, infinite scroll, CRUD mutations) is added in a follow-up task.
 */
export default function CardgroupsClient(_props: CardgroupsClientProps) {
  return null;
}

import type { AdminMasterQuery } from "@/generated/graphql";

/** Non-null master deck shape passed from the RSC page down to the client tree. */
export type AdminMasterDeck = NonNullable<AdminMasterQuery["adminMaster"]>;

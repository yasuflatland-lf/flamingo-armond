import { graphql } from "@/generated";
import type { AdminMasterQuery } from "@/generated/graphql";

/** Single admin master deck incl. DRAFT. Admin-only; non-admin callers receive FORBIDDEN. */
export const AdminMasterQueryDocument = graphql(`
  query AdminMaster($id: ID!) {
    adminMaster(id: $id) {
      id
      name
      description
      language
      level
      category
      coverImageUrl
      source
      version
      status
      isDefaultStarter
      sortOrder
      cardCount
      createdAt
      updatedAt
    }
  }
`);

/** Non-null master deck shape passed from the RSC page down to the client tree. */
export type AdminMasterDeck = NonNullable<AdminMasterQuery["adminMaster"]>;

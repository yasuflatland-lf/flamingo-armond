import { createPersistedQueryLink } from "@apollo/client/link/persisted-queries";
import { sha256Hex } from "./sha256";

export function makeApqLink() {
  return createPersistedQueryLink({
    sha256: sha256Hex,
    useGETForHashedQueries: false,
  });
}

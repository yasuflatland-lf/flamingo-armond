import { uuidv7 } from "uuidv7";

export const REQUEST_ID_HEADER = "X-Request-ID";

export function newRequestId(): string {
  return uuidv7();
}

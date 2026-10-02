import { trimLikeGo } from "@/schemas/go-text";

export function isUserOnboarded(me: { displayName?: string | null } | null | undefined): boolean {
  return me != null && typeof me.displayName === "string" && trimLikeGo(me.displayName).length > 0;
}

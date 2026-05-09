export function isUserOnboarded(me: { displayName?: string | null } | null | undefined): boolean {
  return me != null && typeof me.displayName === "string" && me.displayName.trim().length > 0;
}

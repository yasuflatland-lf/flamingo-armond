/** Route Handler that re-checks a locally-valid session against the backend. */
export const VERIFY_SESSION_PATH = "/auth/verify-session";

/** `/login?reason=` value set after the backend rejected a live session. */
export const SESSION_INVALID_REASON = "session_invalid";

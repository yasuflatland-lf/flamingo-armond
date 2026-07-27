import { RetryLink } from "@apollo/client/link/retry";
import { getMainDefinition } from "@apollo/client/utilities";

/**
 * Retries transport-level failures on a flaky mobile link. Queries only:
 * `handleSwipe` and every other mutation in this app advances server state with
 * no idempotency key, so a retry after a request that reached the backend but
 * whose response was lost would apply the write twice.
 */
export function makeRetryLink() {
  return new RetryLink({
    delay: { initial: 300, max: 3000, jitter: true },
    attempts: {
      max: 3,
      retryIf: (error, operation) => {
        if (error == null) return false;
        const definition = getMainDefinition(operation.query);
        return definition.kind === "OperationDefinition" && definition.operation === "query";
      },
    },
  });
}

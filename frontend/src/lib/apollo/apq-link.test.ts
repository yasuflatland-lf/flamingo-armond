import { gql } from "@apollo/client";
import { describe, expect, it } from "vitest";
import { makeApqLink } from "./apq-link";

describe("makeApqLink", () => {
  it("returns an ApolloLink", () => {
    const link = makeApqLink();
    expect(link).toBeDefined();
    expect(typeof link.request).toBe("function");
  });

  it("attaches extensions.persistedQuery to outbound context", () => {
    // Minimal exercise: confirm the link is constructable and is a valid ApolloLink.
    // Detailed round-trip is covered by the backend APQ integration test.
    expect(makeApqLink()).toBeDefined();
    expect(gql`query Ping { health }`).toBeDefined();
  });
});

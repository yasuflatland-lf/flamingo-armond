/**
 * Shared cardgroup and card fixtures for broad page-level tests.
 *
 * All IDs are deterministic strings and all timestamps are fixed ISO strings
 * so snapshot assertions remain stable across runs.
 */

import {
  type Card,
  type CardConnection,
  type CardEdge,
  type Cardgroup,
  LearnDisplayMode,
  type PageInfo,
} from "@/generated/base-types";

const userCardState = (due: string) => ({
  __typename: "UserCardState" as const,
  due,
  difficulty: 0.3,
  lapses: 0,
  lastReview: "2026-01-15T00:00:00Z",
  reps: 1,
  scheduledDays: 0,
  stability: 1.0,
  state: 0,
});

// ---------------------------------------------------------------------------
// Cardgroup fixture
// ---------------------------------------------------------------------------

/** A single cardgroup owned by the admin user fixture. */
const cardgroupFixture: Cardgroup = {
  __typename: "Cardgroup",
  id: "cardgroup-001",
  name: "Test Cardgroup",
  ownerId: "user-admin-1",
  // owner is a resolved field; omit here as fixtures target flat data payloads.
  // Tests that need the nested owner object should use makeCardgroup with an override.
  owner: {
    __typename: "User",
    id: "user-admin-1",
    version: 0,
    displayName: "Admin User",
    bio: null,
    avatarUrl: null,
    lastSignInAt: null,
    lastViewedCardgroup: null,
    learnDisplayMode: LearnDisplayMode.FlipToReveal,
    newCardRatio: { __typename: "NewCardRatio", numerator: 4, denominator: 5 },
    roles: [],
  },
  createdAt: "2026-01-15T00:00:00Z",
  updatedAt: "2026-01-15T00:00:00Z",
};

// ---------------------------------------------------------------------------
// Card fixtures
// ---------------------------------------------------------------------------

/**
 * A list of 5 cards that belong to `cardgroupFixture`.
 * IDs are zero-padded to three digits so lexicographic and numeric order match.
 */
export const cardsFixture: Card[] = [
  {
    __typename: "Card",
    id: "card-001",
    front: "front-001",
    back: "back-001",
    cefrLevel: null,
    cardgroupId: "cardgroup-001",
    cardgroup: cardgroupFixture,
    createdAt: "2026-01-15T00:00:00Z",
    updatedAt: "2026-01-15T00:00:00Z",
    userCardState: userCardState("2026-02-01T00:00:00Z"),
  },
  {
    __typename: "Card",
    id: "card-002",
    front: "front-002",
    back: "back-002",
    cefrLevel: null,
    cardgroupId: "cardgroup-001",
    cardgroup: cardgroupFixture,
    createdAt: "2026-01-15T01:00:00Z",
    updatedAt: "2026-01-15T01:00:00Z",
    userCardState: userCardState("2026-02-02T00:00:00Z"),
  },
  {
    __typename: "Card",
    id: "card-003",
    front: "front-003",
    back: "back-003",
    cefrLevel: null,
    cardgroupId: "cardgroup-001",
    cardgroup: cardgroupFixture,
    createdAt: "2026-01-15T02:00:00Z",
    updatedAt: "2026-01-15T02:00:00Z",
    userCardState: userCardState("2026-02-03T00:00:00Z"),
  },
  {
    __typename: "Card",
    id: "card-004",
    front: "front-004",
    back: "back-004",
    cefrLevel: null,
    cardgroupId: "cardgroup-001",
    cardgroup: cardgroupFixture,
    createdAt: "2026-01-15T03:00:00Z",
    updatedAt: "2026-01-15T03:00:00Z",
    userCardState: userCardState("2026-02-04T00:00:00Z"),
  },
  {
    __typename: "Card",
    id: "card-005",
    front: "front-005",
    back: "back-005",
    cefrLevel: null,
    cardgroupId: "cardgroup-001",
    cardgroup: cardgroupFixture,
    createdAt: "2026-01-15T04:00:00Z",
    updatedAt: "2026-01-15T04:00:00Z",
    userCardState: userCardState("2026-02-05T00:00:00Z"),
  },
];

// ---------------------------------------------------------------------------
// Factories
// ---------------------------------------------------------------------------

/**
 * Builds an ad-hoc Cardgroup by merging caller-supplied overrides onto the
 * baseline fixture.
 */
export function makeCardgroup(overrides: Partial<Cardgroup> = {}): Cardgroup {
  return {
    ...cardgroupFixture,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Connection builder
// ---------------------------------------------------------------------------

/**
 * Constructs a CardConnection in the Relay Connection shape consumed by the
 * `CardsByCardgroupConnection` query.
 *
 * The `cursor` for each edge equals the card ID (matching the server
 * implementation documented in the pagination rules).
 *
 * @param cards      Cards to include as edges.
 * @param hasNextPage Whether the server has more rows after this page.
 * @param totalCount  Total card count in the cardgroup (independent of the page window).
 */
export function cardsConnectionFixture(
  cards: Card[],
  hasNextPage: boolean,
  totalCount: number,
): CardConnection {
  const edges: CardEdge[] = cards.map((card) => ({
    __typename: "CardEdge",
    cursor: card.id,
    node: card,
  }));

  const pageInfo: PageInfo = {
    __typename: "PageInfo",
    hasNextPage,
    hasPreviousPage: false,
    startCursor: cards[0]?.id ?? null,
    endCursor: cards[cards.length - 1]?.id ?? null,
  };

  return {
    __typename: "CardConnection",
    edges,
    pageInfo,
    totalCount,
  };
}

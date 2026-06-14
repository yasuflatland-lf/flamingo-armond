package domain

// CardgroupID identifies a user Cardgroup aggregate.
//
// The zero value "" is invalid; use a non-empty value obtained from NewID, a
// client-supplied opaque handle, or a uuid DB column. It is deliberately a bare
// newtype with NO Parse constructor: an ID carries no domain-authored invariant
// (UUID validity is guaranteed at generation by NewID and by the uuid DB
// column), so the type's only job is compile-time ID-space tagging to prevent
// passing a UserID where a CardgroupID is expected. A malformed id must collapse
// to not-found at the lookup, never surface as a validation error. Construct via
// a direct cast at boundaries: domain.CardgroupID(s).
//
// The compile-time tag protects more than the domain structs and aggregate
// methods. It also types the authorization-path helper signatures
// (authorizeCardgroupOrBadInput / authorizeCardgroupOrUnauthenticated, whose
// adjacent cardgroup-id and user-id parameters are the transposition-risk site)
// and the HandleSwipeInput.CardgroupID field. The CardgroupOwnershipFinder
// usecase-narrow interface and the underlying repository.CardgroupRepository.FindByID
// method, and non-authz usecase input fields (e.g. CreateCardInput /
// CardConnectionInput / ImportCardsInput CardgroupID), stay raw string by
// design: typing them would fan out through every FindByID caller and the
// master mappers with no authz payoff. The cast is applied at that boundary
// instead (FindByID(ctx, string(id)); domain.CardgroupID(in.CardgroupID)).
//
// Only UserID and CardgroupID are typed (the authorization-confusable pair).
// CardID, RoleID, and master-aggregate IDs stay raw string by design — they are
// not authz-confusable and typing them would fan out through the master mappers
// with no payoff.
type CardgroupID string

// UserID identifies a User (auth.users.id). Same opaque-handle, no-Parse
// contract as CardgroupID; the zero value "" is invalid.
type UserID string

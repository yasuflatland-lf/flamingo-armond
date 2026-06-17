package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/rotisserie/eris"

	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// MasterCardUsecase is the admin-only READ surface for master cards: a single
// master deck (incl. DRAFT) with its card count, and the master deck's cards as
// a Relay-style paginated connection. Master cards carry no per-viewer / FSRS
// state, so neither method takes a user-card argument. Both methods require the
// AdminGate to pass; non-admin callers receive FORBIDDEN, anonymous callers
// receive UNAUTHENTICATED.
type MasterCardUsecase interface {
	// AdminMaster returns the master cardgroup with the given id INCLUDING DRAFT
	// decks, bundled with its current card count. Admin-only. A missing row is a
	// validation error on "id".
	AdminMaster(ctx context.Context, id string) (*MasterWithCount, error)
	// ListMasterCards paginates a master deck's cards with Relay-style forward
	// (first/after) or backward (last/before) cursors. Admin-only.
	ListMasterCards(ctx context.Context, in MasterCardConnectionInput) (*MasterCardConnectionOutput, error)
}

// MasterCardOrderBy mirrors the schema MasterCardOrderBy enum but stays in the
// usecase layer so the repository remains independent of the GraphQL model
// package. The string values are identical to model.MasterCardOrderBy so the
// resolver can convert with a direct cast.
type MasterCardOrderBy string

const (
	MasterCardOrderByID        MasterCardOrderBy = "ID"
	MasterCardOrderByPosition  MasterCardOrderBy = "POSITION"
	MasterCardOrderByCreatedAt MasterCardOrderBy = "CREATED_AT"
	MasterCardOrderByUpdatedAt MasterCardOrderBy = "UPDATED_AT"
)

// MasterCardConnectionInput captures the GraphQL pagination arguments for
// adminMasterCardsConnection. Pointer fields preserve "absent" semantics from
// the schema so the usecase can default unset values explicitly.
type MasterCardConnectionInput struct {
	MasterCardgroupID string
	First, Last       *int
	After, Before     *string // raw GraphQL ID strings (cursor = master card UUID)
	// Search is optional; nil disables the filter. The usecase normalizes
	// whitespace-only strings to nil before reaching the repository.
	Search         *string
	OrderBy        *MasterCardOrderBy
	OrderDirection *SortOrder
}

// MasterCardConnectionOutput is the usecase-level page result. The resolver
// wraps it into a model.MasterCardConnection.
type MasterCardConnectionOutput struct {
	Cards      []*domain.MasterCard
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
}

type masterCardUsecase struct {
	masterCardRepo      repository.MasterCardRepository
	masterCardgroupRepo repository.MasterCardgroupRepository
	adminGate           *AdminGate
	logger              *slog.Logger
}

// NewMasterCardUsecase constructs a MasterCardUsecase. adminGate gates every
// method. Panics when any dependency is nil — a nil required dependency is a
// wiring bug that must fail at startup, not at first use.
func NewMasterCardUsecase(
	masterCard repository.MasterCardRepository,
	masterCardgroup repository.MasterCardgroupRepository,
	adminGate *AdminGate,
	logger *slog.Logger,
) MasterCardUsecase {
	if masterCard == nil {
		panic("usecase: master card: masterCard repository is required")
	}
	if masterCardgroup == nil {
		panic("usecase: master card: masterCardgroup repository is required")
	}
	if adminGate == nil {
		panic("usecase: master card: adminGate is required")
	}
	if logger == nil {
		panic("usecase: master card: logger is required")
	}
	return &masterCardUsecase{
		masterCardRepo:      masterCard,
		masterCardgroupRepo: masterCardgroup,
		adminGate:           adminGate,
		logger:              logger,
	}
}

// AdminMaster returns the master cardgroup (incl. DRAFT) with the given id plus
// its card count. Admin-only: the gate rejects non-admin / anonymous callers
// before any repository access. FindByID returns ANY status, so DRAFT decks are
// included. A missing row surfaces as a validation error on "id".
func (u *masterCardUsecase) AdminMaster(ctx context.Context, id string) (*MasterWithCount, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: admin master"); err != nil {
		return nil, err
	}
	master, err := u.masterCardgroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError("id", "master cardgroup not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: admin master: find by id")
	}
	count, err := u.masterCardgroupRepo.CountCards(ctx, id)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: admin master: count cards")
	}
	return &MasterWithCount{Master: master, CardCount: count}, nil
}

// ListMasterCards paginates a master deck's cards using Relay-style forward
// (first/after) or backward (last/before) cursors. Admin-only. The mixed
// direction combinations are rejected with BAD_USER_INPUT before any repository
// access. totalCount comes from a separate COUNT(*) computed before the
// first==0 && last==0 short-circuit so a totalCount-only request still observes
// the real count.
func (u *masterCardUsecase) ListMasterCards(
	ctx context.Context, in MasterCardConnectionInput,
) (*MasterCardConnectionOutput, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master card: list"); err != nil {
		return nil, err
	}

	first, last, err := resolveRelayPage(in.First, in.Last, in.After, in.Before, resolveStandardPageSize)
	if err != nil {
		return nil, err
	}

	orderBy, dir, err := resolveMasterCardOrderBy(in.OrderBy, in.OrderDirection)
	if err != nil {
		return nil, err
	}

	after, err := u.resolveMasterCardCursor(ctx, in.After, in.MasterCardgroupID, orderBy, "after")
	if err != nil {
		return nil, err
	}
	before, err := u.resolveMasterCardCursor(ctx, in.Before, in.MasterCardgroupID, orderBy, "before")
	if err != nil {
		return nil, err
	}

	// Normalize: nil and whitespace-only both mean "no filter". After this block
	// a non-nil search pointer holds a non-empty, trimmed string — the repository
	// relies on this invariant.
	search := in.Search
	if search != nil {
		trimmed := strings.TrimSpace(*search)
		if trimmed == "" {
			search = nil
		} else {
			search = &trimmed
		}
	}

	// totalCount via a separate COUNT(*), computed before the page fetch so a
	// totalCount-only request (first==0 && last==0) still observes the real count.
	total, err := u.masterCardRepo.CountByMasterCardgroup(ctx, in.MasterCardgroupID)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: list: count by master cardgroup")
	}

	cards, hasNext, hasPrev, err := assemblePage(first, last, after != nil, before != nil,
		func(wantFirst, wantLast int) ([]*domain.MasterCard, error) {
			rows, _, e := u.masterCardRepo.FindPageByMasterCardgroup(
				ctx, in.MasterCardgroupID, after, before, wantFirst, wantLast, orderBy, dir, search,
			)
			if e != nil {
				if isContextDone(e) {
					return nil, e
				}
				return nil, eris.Wrap(e, "usecase: master card: list: find page")
			}
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	out := &MasterCardConnectionOutput{TotalCount: total, HasNext: hasNext, HasPrev: hasPrev, Cards: cards}
	if len(cards) > 0 {
		out.StartCur = cursor.Encode(cards[0].ID)
		out.EndCur = cursor.Encode(cards[len(cards)-1].ID)
	}
	return out, nil
}

// resolveMasterCardOrderBy maps the typed usecase enums to the repository
// allowlist. Defaults match the schema (POSITION, ASC) when both inputs are
// nil. The default switch arm is defense in depth — gqlgen UnmarshalGQL already
// rejects invalid enum strings upstream.
func resolveMasterCardOrderBy(
	orderBy *MasterCardOrderBy, dir *SortOrder,
) (repository.MasterCardOrderBy, repository.SortOrder, error) {
	field := repository.MasterCardOrderByPosition
	if orderBy != nil {
		switch *orderBy {
		case MasterCardOrderByID:
			field = repository.MasterCardOrderByID
		case MasterCardOrderByPosition:
			field = repository.MasterCardOrderByPosition
		case MasterCardOrderByCreatedAt:
			field = repository.MasterCardOrderByCreatedAt
		case MasterCardOrderByUpdatedAt:
			field = repository.MasterCardOrderByUpdatedAt
		default:
			return "", "", ucerr.NewValidationError("orderBy", "invalid")
		}
	}
	d := repository.SortAsc
	if dir != nil {
		switch *dir {
		case SortOrderAsc:
			d = repository.SortAsc
		case SortOrderDesc:
			d = repository.SortDesc
		default:
			return "", "", ucerr.NewValidationError("orderDirection", "invalid")
		}
	}
	return field, d, nil
}

// resolveMasterCardCursor decodes an opaque cursor string into a
// *repository.MasterCardCursor with the column required by the active orderBy
// populated. Returns BAD_USER_INPUT when the cursor cannot be decoded, the
// master card cannot be found, or it belongs to a different master cardgroup.
//
// For MasterCardOrderByID no column hydration is needed — the decoded id is the
// full cursor. For the time/position orderings the column value must be
// hydrated; the MasterCardRepository contract offers no single-card lookup, so
// the cursor row is located within ListByMasterCardgroup (master decks are
// bounded admin templates). A missing column for the active orderBy is a
// caller/internal bug surfaced as an error, never a silent zero-value (which
// would generate a wrong-but-valid SQL predicate and quietly skip rows).
func (u *masterCardUsecase) resolveMasterCardCursor(
	ctx context.Context,
	cursorStr *string,
	masterCardgroupID string,
	orderBy repository.MasterCardOrderBy,
	field string,
) (*repository.MasterCardCursor, error) {
	if cursorStr == nil || *cursorStr == "" {
		return nil, nil
	}
	id, err := cursor.Decode(*cursorStr)
	if err != nil {
		return nil, ucerr.NewValidationError(field, "invalid cursor")
	}
	c := &repository.MasterCardCursor{ID: id}
	if orderBy == repository.MasterCardOrderByID {
		return c, nil
	}

	card, err := u.findMasterCardInGroup(ctx, masterCardgroupID, id)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, ucerr.NewValidationError(field, "cursor not found")
	}

	switch orderBy {
	case repository.MasterCardOrderByPosition:
		pos := card.Position
		c.Position = &pos
	case repository.MasterCardOrderByCreatedAt:
		ca := card.CreatedAt
		c.CreatedAt = &ca
	case repository.MasterCardOrderByUpdatedAt:
		ua := card.UpdatedAt
		c.UpdatedAt = &ua
	default:
		return nil, eris.Errorf("usecase: master card: unhandled orderBy %q", orderBy)
	}
	return c, nil
}

// findMasterCardInGroup returns the master card with the given id from the
// supplied group, or (nil, nil) when no card in the group matches. The
// MasterCardRepository contract has no single-card lookup; ListByMasterCardgroup
// scoped to the group is the cross-aggregate guard (a cursor id from a different
// deck never appears in the result and so is treated as not-found).
func (u *masterCardUsecase) findMasterCardInGroup(
	ctx context.Context, masterCardgroupID, id string,
) (*domain.MasterCard, error) {
	cards, err := u.masterCardRepo.ListByMasterCardgroup(ctx, masterCardgroupID)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: master card: hydrate cursor: list by master cardgroup")
	}
	for _, card := range cards {
		if card.ID == id {
			return card, nil
		}
	}
	return nil, nil
}

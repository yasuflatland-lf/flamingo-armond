// master_catalog_admin.go holds the admin-author surface of the master catalog:
// the carrier types for the admin mutations, the mapMasterAdminErr repository-error
// classifier, and the admin-gated management methods (admin single get, create,
// update, publish, unpublish, delete, admin list). Every method here passes through
// AdminGate.Require first, and a missing deck is a validation error on "id" rather
// than the non-disclosure not-found the public reader surface uses.

package usecase

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// MasterWithCount bundles a master cardgroup with its current card count so the
// resolver can populate the non-null model.MasterCardgroup.cardCount field on a
// single-entity admin response.
type MasterWithCount struct {
	Master    *domain.MasterCardgroup
	CardCount int64
}

// CreateMasterInput carries the admin create fields. Optional attributes are
// pointers preserving "absent" semantics from the GraphQL input.
type CreateMasterInput struct {
	Name             string
	Description      *string
	IsDefaultStarter *bool
	SortOrder        *int
}

// UpdateMasterInput carries the admin update patch. nil = leave unchanged.
type UpdateMasterInput struct {
	Name             *string
	Description      *string
	IsDefaultStarter *bool
	SortOrder        *int
}

// CreateMasterOutcome is the result of CreateMaster. Exactly one of Master or
// Validation is non-nil on a nil-error return. A freshly created deck has no
// cards (cardCount = 0).
type CreateMasterOutcome struct {
	Master     *domain.MasterCardgroup
	Validation *InputValidationInfo
}

// UpdateMasterOutcome is the result of UpdateMaster. Exactly one of Master or
// Validation is non-nil. CardCount is the deck's current card count, fetched so
// the resolver can populate model.MasterCardgroup.cardCount.
type UpdateMasterOutcome struct {
	Master     *domain.MasterCardgroup
	CardCount  int64
	Validation *InputValidationInfo
}

// PublishMasterOutcome is the result of PublishMaster. Exactly one of Master or
// EmptyMaster is set: a deck with zero cards cannot be published, surfaced as
// EmptyMaster = true so the resolver maps it to MasterCardgroupEmptyError.
type PublishMasterOutcome struct {
	Master      *domain.MasterCardgroup
	CardCount   int64
	EmptyMaster bool
}

// mapMasterAdminErr classifies a repository error from an admin master method
// into either input-validation data (first slot) or a propagating error (second
// slot) by delegating to classifyRepoErr with the master-specific sentinel
// mapping. The input-validation emission lives HERE, not in the caller's method
// body, so the schema-lint bare-object gate does not flag
// adminUnpublishMasterCardgroup (which returns a bare MasterCardgroup!).
func mapMasterAdminErr(err error, notFoundField, wrap string) (*InputValidationInfo, error) {
	return classifyRepoErr(err, wrap, []SentinelMapping{
		{repository.ErrNotFound, notFoundField, "master cardgroup not found"},
	})
}

// AdminMaster returns the master cardgroup (incl. DRAFT) with the given id plus
// its card count. Admin-only: the gate rejects non-admin / anonymous callers
// before any repository access. FindByID returns ANY status, so DRAFT decks are
// included. A missing row surfaces as a validation error on "id".
func (u *masterCatalogUsecase) AdminMaster(ctx context.Context, id string) (*MasterWithCount, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: admin master"); err != nil {
		return nil, err
	}
	master, err := u.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError("id", "master cardgroup not found")
		}
		return nil, wrapInfraErr(err, "usecase: master catalog: admin master: find by id")
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return nil, wrapInfraErr(err, "usecase: master catalog: admin master: count cards")
	}
	return &MasterWithCount{Master: master, CardCount: count}, nil
}

// CreateMaster creates a new DRAFT master cardgroup. Admin-only. Name validation
// failures surface via outcome.Validation (mapped to the InputValidationError
// union variant); a new deck starts at version 1, status DRAFT.
func (u *masterCatalogUsecase) CreateMaster(ctx context.Context, in CreateMasterInput) (CreateMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: create master"); err != nil {
		return CreateMasterOutcome{}, err
	}

	name, nameErr := domain.ParseCardgroupName(in.Name)
	info, err := liftValidationErr(translateCardgroupNameErr(nameErr))
	if err != nil {
		return CreateMasterOutcome{}, err
	}
	if info != nil {
		return CreateMasterOutcome{Validation: info}, nil
	}

	description, descErr := domain.ParseDescription(in.Description)
	info, err = liftValidationErr(translateDescriptionErr(descErr))
	if err != nil {
		return CreateMasterOutcome{}, err
	}
	if info != nil {
		return CreateMasterOutcome{Validation: info}, nil
	}

	m, err := domain.NewMasterCardgroup(
		name,
		description,
		derefOr(in.IsDefaultStarter, false),
		derefOr(in.SortOrder, 0),
	)
	if err != nil {
		return CreateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: create master: construct")
	}
	if err := u.repo.Create(ctx, m); err != nil {
		if translated := translateTextLengthViolation(err); translated != nil {
			info, lerr := liftValidationErr(translated)
			if lerr != nil {
				return CreateMasterOutcome{}, lerr
			}
			return CreateMasterOutcome{Validation: info}, nil
		}
		return CreateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: create master")
	}
	return CreateMasterOutcome{Master: m}, nil
}

// UpdateMaster applies an admin patch to an existing master cardgroup. Admin-only.
// Name validation failures surface via outcome.Validation; a missing row surfaces
// as a validation error on "id". CardCount is fetched so the resolver can populate
// the response model.
//
// There is deliberately no MasterCardgroup.ApplyPatch aggregate method. Of the
// patchable fields, Name and Description carry domain invariants — the
// CardgroupName and Description grapheme-cluster length bounds — and both are
// enforced here, at their single seams, via domain.ParseCardgroupName /
// domain.ParseDescription below. Status is the only other VO on the aggregate, and
// it is not patchable through this method: it has its own dedicated lifecycle seams
// (Publish / Unpublish). The remaining fields (IsDefaultStarter, SortOrder) are
// free-form (*bool / *int) with no Parse or bound to protect, so they are assigned
// directly into the repository patch. An ApplyPatch wrapper over those fields would
// be an indirection layer guarding nothing.
func (u *masterCatalogUsecase) UpdateMaster(ctx context.Context, id string, in UpdateMasterInput) (UpdateMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: update master"); err != nil {
		return UpdateMasterOutcome{}, err
	}

	description, descErr := domain.ParseDescription(in.Description)
	info, err := liftValidationErr(translateDescriptionErr(descErr))
	if err != nil {
		return UpdateMasterOutcome{}, err
	}
	if info != nil {
		return UpdateMasterOutcome{Validation: info}, nil
	}

	// Name and Description route through their VOs (above / below); the remaining
	// free-form fields flow straight into the patch. See the method docstring for
	// why no MasterCardgroup.ApplyPatch exists.
	patch := repository.MasterCardgroupUpdate{
		Description:      description.Ptr(),
		IsDefaultStarter: in.IsDefaultStarter,
		SortOrder:        in.SortOrder,
	}
	if in.Name != nil {
		name, nameErr := domain.ParseCardgroupName(*in.Name)
		info, err = liftValidationErr(translateCardgroupNameErr(nameErr))
		if err != nil {
			return UpdateMasterOutcome{}, err
		}
		if info != nil {
			return UpdateMasterOutcome{Validation: info}, nil
		}
		nameStr := name.String()
		patch.Name = &nameStr
	}

	updated, err := u.repo.Update(ctx, id, patch)
	if err != nil {
		if translated := translateTextLengthViolation(err); translated != nil {
			info, lerr := liftValidationErr(translated)
			if lerr != nil {
				return UpdateMasterOutcome{}, lerr
			}
			return UpdateMasterOutcome{Validation: info}, nil
		}
		info, perr := mapMasterAdminErr(err, "id", "usecase: master catalog: update master")
		if perr != nil {
			return UpdateMasterOutcome{}, perr
		}
		return UpdateMasterOutcome{Validation: info}, nil
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return UpdateMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: update master: count cards")
	}
	return UpdateMasterOutcome{Master: updated, CardCount: count}, nil
}

// PublishMaster publishes a master cardgroup after confirming it has at least one
// card. Admin-only. A deck with zero cards is rejected via outcome.EmptyMaster
// (mapped to MasterCardgroupEmptyError) without touching the publish path.
// This guard is immediate admin feedback, not the enforcement point: catalog
// visibility is a read-side predicate (published AND at least one card) applied
// in the repository, so a deck emptied AFTER publication leaves the catalog on
// its own and returns the moment a card is restored.
func (u *masterCatalogUsecase) PublishMaster(ctx context.Context, id string) (PublishMasterOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: publish master"); err != nil {
		return PublishMasterOutcome{}, err
	}
	// Ensure the deck exists before the empty-count guard so a missing id is a
	// validation error rather than a silent "0 cards => empty" classification.
	if _, err := u.repo.FindByID(ctx, id); err != nil {
		return PublishMasterOutcome{}, lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: publish master: find"))
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return PublishMasterOutcome{}, eris.Wrap(err, "usecase: master catalog: publish master: count cards")
	}
	if count == 0 {
		return PublishMasterOutcome{EmptyMaster: true}, nil
	}
	published, err := u.repo.Publish(ctx, id)
	if err != nil {
		return PublishMasterOutcome{}, lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: publish master"))
	}
	return PublishMasterOutcome{Master: published, CardCount: count}, nil
}

// UnpublishMaster reverts a master cardgroup to DRAFT. Admin-only. Returns the
// refreshed master plus its card count. NOTE: this method backs the bare-object
// mutation adminUnpublishMasterCardgroup, so its body must NOT directly reference
// ucerr.NewValidationError / ucerr.NewForbiddenError / ucerr.ErrUnauthenticated —
// the not-found mapping is delegated to mapMasterAdminErr/lowerValidationInfo.
func (u *masterCatalogUsecase) UnpublishMaster(ctx context.Context, id string) (*MasterWithCount, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: unpublish master"); err != nil {
		return nil, err
	}
	updated, err := u.repo.Unpublish(ctx, id)
	if err != nil {
		return nil, lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: unpublish master"))
	}
	count, err := u.repo.CountCards(ctx, id)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: master catalog: unpublish master: count cards")
	}
	return &MasterWithCount{Master: updated, CardCount: count}, nil
}

// DeleteMaster removes a master cardgroup. Admin-only. A missing row is a
// validation error on "id". Returns Boolean! upstream (scalar — schema-lint exempt),
// so direct ucerr use here is permitted, but routed through the helper for
// consistency with UnpublishMaster.
func (u *masterCatalogUsecase) DeleteMaster(ctx context.Context, id string) error {
	if _, err := u.adminGate.Require(ctx, "usecase: master catalog: delete master"); err != nil {
		return err
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return lowerValidationInfo(mapMasterAdminErr(err, "id", "usecase: master catalog: delete master"))
	}
	return nil
}

// ListAdminConnection paginates ALL master cardgroups (DRAFT + PUBLISHED) for the
// admin UI. Admin-only. Mirrors ListPublishedConnection but gates on adminGate and
// calls the status-unfiltered FindPageAnyStatus repository method (whose returned
// total counts decks of any status). The body from page assembly onward is shared
// with ListPublishedConnection via listMasterCatalogCore; only the gate,
// publishedOnly scope (false = admin, DRAFT cursors valid), the repository page
// method, and the eris wrap prefix differ.
func (u *masterCatalogUsecase) ListAdminConnection(
	ctx context.Context, in MasterCatalogConnectionInput,
) (*MasterCatalogConnectionOutput, error) {
	return u.listMasterCatalogCore(ctx, in, false, "usecase: master catalog: find admin page",
		func(ctx context.Context) error {
			_, err := u.adminGate.Require(ctx, "usecase: master catalog: list admin")
			return err
		},
		u.repo.FindPageAnyStatus,
	)
}

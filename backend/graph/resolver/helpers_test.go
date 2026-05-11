package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"backend/internal/domain"
)

// TestToRoleModels_FiltersNil verifies that toRoleModels skips nil domain.Role
// values and returns only non-nil results. This is critical because the
// User.roles field declares [Role!]!, so a nil entry in the list would violate
// the schema.
func TestToRoleModels_FiltersNil(t *testing.T) {
	t.Parallel()

	validRole := &domain.Role{ID: "r1", Name: "admin"}
	roles := []*domain.Role{nil, validRole, nil}

	result := toRoleModels(roles)

	assert.Len(t, result, 1)
	if len(result) > 0 {
		assert.Equal(t, "r1", result[0].ID)
		assert.Equal(t, "admin", result[0].Name)
	}
}

// TestToCardModels_FiltersNil verifies that toCardModels skips nil domain.Card
// values and returns only non-nil results. This is critical because
// Query.cardsByCardgroup declares [Card!]!, so a nil entry would violate
// the schema.
func TestToCardModels_FiltersNil(t *testing.T) {
	t.Parallel()

	validCard := &domain.Card{
		ID:          "c1",
		Front:       "Q",
		Back:        "A",
		CardgroupID: "cg1",
	}
	cards := []*domain.Card{nil, validCard, nil}

	result := toCardModels(cards)

	assert.Len(t, result, 1)
	if len(result) > 0 {
		assert.Equal(t, "c1", result[0].ID)
		assert.Equal(t, "Q", result[0].Front)
		assert.Equal(t, "A", result[0].Back)
	}
}

// TestToCardgroupModels_FiltersNil verifies that toCardgroupModels skips nil
// domain.Cardgroup values and returns only non-nil results. This is critical
// because Query.myCardgroups declares [Cardgroup!]!, so a nil entry would
// violate the schema.
func TestToCardgroupModels_FiltersNil(t *testing.T) {
	t.Parallel()

	validCardgroup := &domain.Cardgroup{
		ID:      "cg1",
		Name:    "Spanish Vocab",
		OwnerID: "u1",
	}
	cardgroups := []*domain.Cardgroup{nil, validCardgroup, nil}

	result := toCardgroupModels(cardgroups)

	assert.Len(t, result, 1)
	if len(result) > 0 {
		assert.Equal(t, "cg1", result[0].ID)
		assert.Equal(t, "Spanish Vocab", result[0].Name)
		assert.Equal(t, "u1", result[0].OwnerID)
	}
}

// TestToRoleModels_AllNil verifies the edge case where all input roles are nil.
func TestToRoleModels_AllNil(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{nil, nil, nil}
	result := toRoleModels(roles)

	assert.Empty(t, result)
}

// TestToRoleModels_Empty verifies the edge case where the input slice is empty.
func TestToRoleModels_Empty(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{}
	result := toRoleModels(roles)

	assert.Empty(t, result)
}

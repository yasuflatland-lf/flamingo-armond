package repository

import "testing"

func TestCardRepositorySatisfiesNarrowInterfaces(t *testing.T) {
	repo := NewCardRepository(nil)

	var _ CardReadRepository = repo
	var _ CardPageRepository = repo
	var _ CardWriteRepository = repo
	var _ CardRepository = repo
}

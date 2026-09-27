package repository

import (
	"math"
	"testing"
)

func TestBulkStatementChunk_BindsUnderPgxLimit(t *testing.T) {
	t.Parallel()
	if got := bulkStatementChunkRows * upsertParamsPerRow; got > math.MaxUint16 {
		t.Fatalf("upsert chunk binds %d parameters, pgx limit is %d", got, math.MaxUint16)
	}
	if got := bulkStatementChunkRows + 1; got > math.MaxUint16 {
		t.Fatalf("fold chunk binds %d parameters, pgx limit is %d", got, math.MaxUint16)
	}
}

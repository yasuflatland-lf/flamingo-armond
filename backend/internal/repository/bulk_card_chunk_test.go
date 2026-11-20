package repository

import (
	"math"
	"strings"
	"testing"
)

func TestBulkStatementChunk_BindsUnderPgxLimit(t *testing.T) {
	t.Parallel()
	if got := strings.Count(upsertRowPlaceholders, "?"); got != upsertParamsPerRow {
		t.Fatalf("upsert row binds %d parameters, upsertParamsPerRow is %d", got, upsertParamsPerRow)
	}
	if got := bulkStatementChunkRows * upsertParamsPerRow; got > math.MaxUint16 {
		t.Fatalf("upsert chunk binds %d parameters, pgx limit is %d", got, math.MaxUint16)
	}
	if got := bulkStatementChunkRows + 1; got > math.MaxUint16 {
		t.Fatalf("fronts chunk binds %d parameters, pgx limit is %d", got, math.MaxUint16)
	}
}

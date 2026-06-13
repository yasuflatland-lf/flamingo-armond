// Command migrate-to-master is a one-time data migration CLI that copies an
// owner's existing personal deck (public.cardgroups / public.cards) into the
// admin-curated master catalog tables (public.master_cardgroups /
// public.master_cards).
//
// The personal cardgroups / cards rows are KEPT — this tool never deletes them,
// so the owner's FSRS history (public.user_card_fsrs, keyed by card id) is
// preserved. The copy snapshots each personal cardgroup as a published default
// starter deck (status='published', is_default_starter=true, source='notion',
// version=1, sort_order=0) and each personal card as a master card.
//
// The migration is idempotent: master rows reuse the source row's primary key
// and upsert via ON CONFLICT (id) DO UPDATE, so re-running converges on the same
// state rather than duplicating rows.
//
// Subcommands:
//
//	run    --db-url <superuser DSN> --owner-email <email>
//	       Resolve the owner email to its auth.users UUID, then upsert that
//	       owner's cardgroups and cards into the master tables. Prints counts.
//
//	verify --db-url <superuser DSN> --owner-email <email>
//	       Read-only. Compares master_cardgroups / master_cards counts against
//	       the owner's public.cardgroups / public.cards counts and prints parity.
//
// The superuser DSN is required because resolving --owner-email reads auth.users,
// which is owned by supabase_auth_admin and not readable by the application role.
// See docs/backend/library-gotchas/raw-sql-cli-pgx-stdlib.md.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rotisserie/eris"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// cardgroupRow mirrors the columns read from public.cardgroups.
type cardgroupRow struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// cardRow mirrors the columns read from public.cards.
type cardRow struct {
	ID          string
	CardgroupID string
	Front       string
	Back        string
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func main() {
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	runDBURL := runCmd.String("db-url", "", "superuser database connection URL (required; reads auth.users)")
	runOwnerEmail := runCmd.String("owner-email", "", "email of the owner whose deck is copied into the master catalog")

	verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
	verifyDBURL := verifyCmd.String("db-url", "", "superuser database connection URL (required; reads auth.users)")
	verifyOwnerEmail := verifyCmd.String("owner-email", "", "email of the owner whose parity is checked")

	if len(os.Args) < 2 {
		log.Fatal("usage: migrate-to-master <run|verify> --db-url <dsn> --owner-email <email>")
	}

	switch os.Args[1] {
	case "run":
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(eris.ToString(eris.Wrap(err, "migrate: parse run flags"), true))
		}
		if err := runMigrate(*runDBURL, *runOwnerEmail); err != nil {
			log.Fatal(eris.ToString(err, true))
		}
	case "verify":
		if err := verifyCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(eris.ToString(eris.Wrap(err, "migrate: parse verify flags"), true))
		}
		if err := runVerify(*verifyDBURL, *verifyOwnerEmail); err != nil {
			log.Fatal(eris.ToString(err, true))
		}
	default:
		log.Fatalf("unknown subcommand %q (want run|verify)", os.Args[1])
	}
}

// openDB validates the flags, opens the connection, and pings it. The DSN must
// be validated before sql.Open because sql.Open("pgx", "") succeeds (the driver
// defers the connection), so the first observable failure would otherwise be a
// confusing Ping error rather than a clear "missing flag" message.
func openDB(dbURL, ownerEmail string) (*sql.DB, error) {
	if dbURL == "" {
		return nil, eris.New("migrate: --db-url is required (superuser DSN; reads auth.users)")
	}
	if ownerEmail == "" {
		return nil, eris.New("migrate: --owner-email is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, eris.Wrap(err, "migrate: open db")
	}
	if err := db.Ping(); err != nil {
		if cerr := db.Close(); cerr != nil {
			log.Printf("migrate: db.Close after ping failure: %v", cerr)
		}
		return nil, eris.Wrap(err, "migrate: ping db")
	}
	return db, nil
}

// resolveOwnerID looks up the owner's auth.users UUID by case-insensitive email.
func resolveOwnerID(db *sql.DB, ownerEmail string) (string, error) {
	var ownerID string
	err := db.QueryRow(
		`SELECT id FROM auth.users WHERE lower(email) = lower($1)`,
		ownerEmail,
	).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return "", eris.Errorf("migrate: no auth.users row for owner email %q", ownerEmail)
	}
	if err != nil {
		return "", eris.Wrap(err, "migrate: resolve owner email")
	}
	return ownerID, nil
}

// readCardgroups reads every public.cardgroups row owned by ownerID.
func readCardgroups(db *sql.DB, ownerID string) ([]cardgroupRow, error) {
	rows, err := db.Query(
		`SELECT id, name, created_at, updated_at
		   FROM public.cardgroups
		  WHERE owner_id = $1
		  ORDER BY created_at, id`,
		ownerID,
	)
	if err != nil {
		return nil, eris.Wrap(err, "migrate: query cardgroups")
	}
	defer rows.Close()

	var out []cardgroupRow
	for rows.Next() {
		var cg cardgroupRow
		if err := rows.Scan(&cg.ID, &cg.Name, &cg.CreatedAt, &cg.UpdatedAt); err != nil {
			return nil, eris.Wrap(err, "migrate: scan cardgroup row")
		}
		out = append(out, cg)
	}
	if err := rows.Err(); err != nil {
		return nil, eris.Wrap(err, "migrate: iterate cardgroup rows")
	}
	rows.Close()
	return out, nil
}

// readCards reads every public.cards row whose cardgroup_id is in cardgroupIDs.
func readCards(db *sql.DB, cardgroupIDs []string) ([]cardRow, error) {
	if len(cardgroupIDs) == 0 {
		return nil, nil
	}
	rows, err := db.Query(
		`SELECT id, cardgroup_id, front, back, position, created_at, updated_at
		   FROM public.cards
		  WHERE cardgroup_id = ANY($1)
		  ORDER BY cardgroup_id, position, id`,
		cardgroupIDs,
	)
	if err != nil {
		return nil, eris.Wrap(err, "migrate: query cards")
	}
	defer rows.Close()

	var out []cardRow
	for rows.Next() {
		var c cardRow
		if err := rows.Scan(&c.ID, &c.CardgroupID, &c.Front, &c.Back, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, eris.Wrap(err, "migrate: scan card row")
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, eris.Wrap(err, "migrate: iterate card rows")
	}
	rows.Close()
	return out, nil
}

// runMigrate copies the owner's personal deck into the master catalog tables.
// All upserts run inside one transaction so the migration is all-or-nothing.
func runMigrate(dbURL, ownerEmail string) (retErr error) {
	db, err := openDB(dbURL, ownerEmail)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("migrate: db.Close: %v", err)
		}
	}()

	ownerID, err := resolveOwnerID(db, ownerEmail)
	if err != nil {
		return err
	}

	cardgroups, err := readCardgroups(db, ownerID)
	if err != nil {
		return err
	}

	cardgroupIDs := make([]string, 0, len(cardgroups))
	for _, cg := range cardgroups {
		cardgroupIDs = append(cardgroupIDs, cg.ID)
	}

	cards, err := readCards(db, cardgroupIDs)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return eris.Wrap(err, "migrate: begin tx")
	}
	defer func() {
		if retErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("migrate: tx.Rollback failed: %v (original error: %v)", rbErr, retErr)
			}
		}
	}()

	// master_cardgroups upsert. The source cardgroup id is preserved as the
	// master_cardgroups id so master_cards can reference it and so re-runs
	// converge via ON CONFLICT (id). The catalog metadata (source, version,
	// status, is_default_starter, sort_order) is fixed for this initial copy.
	cgUpserted := 0
	for _, cg := range cardgroups {
		_, err = tx.Exec(`
			INSERT INTO public.master_cardgroups
			  (id, name, source, version, status, is_default_starter, sort_order, created_at, updated_at)
			VALUES ($1, $2, 'notion', 1, 'published', true, 0, $3, $4)
			ON CONFLICT (id) DO UPDATE
			  SET name               = EXCLUDED.name,
			      source             = EXCLUDED.source,
			      version            = EXCLUDED.version,
			      status             = EXCLUDED.status,
			      is_default_starter = EXCLUDED.is_default_starter,
			      sort_order         = EXCLUDED.sort_order,
			      updated_at         = EXCLUDED.updated_at`,
			cg.ID, cg.Name, cg.CreatedAt, cg.UpdatedAt,
		)
		if err != nil {
			return eris.Wrapf(err, "migrate: upsert master_cardgroup %s", cg.ID)
		}
		cgUpserted++
	}

	// master_cards upsert. master_cardgroup_id is the source card's
	// cardgroup_id, which now exists in master_cardgroups (same id, upserted
	// above), so the FK is satisfied.
	cardUpserted := 0
	for _, c := range cards {
		_, err = tx.Exec(`
			INSERT INTO public.master_cards
			  (id, master_cardgroup_id, front, back, position, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE
			  SET master_cardgroup_id = EXCLUDED.master_cardgroup_id,
			      front               = EXCLUDED.front,
			      back                = EXCLUDED.back,
			      position            = EXCLUDED.position,
			      updated_at          = EXCLUDED.updated_at`,
			c.ID, c.CardgroupID, c.Front, c.Back, c.Position, c.CreatedAt, c.UpdatedAt,
		)
		if err != nil {
			return eris.Wrapf(err, "migrate: upsert master_card %s", c.ID)
		}
		cardUpserted++
	}

	if err = tx.Commit(); err != nil {
		return eris.Wrap(err, "migrate: commit transaction")
	}

	fmt.Printf("migrate run complete: %d master_cardgroups, %d master_cards upserted (owner %s)\n",
		cgUpserted, cardUpserted, ownerEmail)
	return nil
}

// runVerify is read-only. It compares the owner's personal deck counts against
// the master catalog counts and prints parity. Because the master tables hold
// every owner's published decks (not just this owner's), exact equality is only
// meaningful when the catalog was seeded solely from this owner's deck; the
// report therefore prints both counts and flags whether master >= personal.
func runVerify(dbURL, ownerEmail string) error {
	db, err := openDB(dbURL, ownerEmail)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("migrate: db.Close: %v", err)
		}
	}()

	ownerID, err := resolveOwnerID(db, ownerEmail)
	if err != nil {
		return err
	}

	cardgroups, err := readCardgroups(db, ownerID)
	if err != nil {
		return err
	}
	cardgroupIDs := make([]string, 0, len(cardgroups))
	for _, cg := range cardgroups {
		cardgroupIDs = append(cardgroupIDs, cg.ID)
	}
	cards, err := readCards(db, cardgroupIDs)
	if err != nil {
		return err
	}

	// Count master rows whose id matches the owner's personal rows. This is the
	// exact set this tool's run would have copied, so it is the right parity
	// scope even when the catalog holds other decks.
	var masterCG int
	if len(cardgroupIDs) > 0 {
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM public.master_cardgroups WHERE id = ANY($1)`,
			cardgroupIDs,
		).Scan(&masterCG); err != nil {
			return eris.Wrap(err, "migrate: count master_cardgroups")
		}
	}

	cardIDs := make([]string, 0, len(cards))
	for _, c := range cards {
		cardIDs = append(cardIDs, c.ID)
	}
	var masterCards int
	if len(cardIDs) > 0 {
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM public.master_cards WHERE id = ANY($1)`,
			cardIDs,
		).Scan(&masterCards); err != nil {
			return eris.Wrap(err, "migrate: count master_cards")
		}
	}

	cgParity := masterCG == len(cardgroups)
	cardParity := masterCards == len(cards)

	fmt.Printf("migrate verify (owner %s):\n", ownerEmail)
	fmt.Printf("  cardgroups: personal=%d master=%d parity=%t\n", len(cardgroups), masterCG, cgParity)
	fmt.Printf("  cards:      personal=%d master=%d parity=%t\n", len(cards), masterCards, cardParity)

	if !cgParity || !cardParity {
		return eris.Errorf("migrate: parity mismatch (cardgroups %d/%d, cards %d/%d) — run the migration first",
			masterCG, len(cardgroups), masterCards, len(cards))
	}
	fmt.Println("  parity OK")
	return nil
}

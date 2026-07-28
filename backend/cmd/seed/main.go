package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rotisserie/eris"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DumpFile struct {
	Version      int         `json:"version"`
	DumpedAt     time.Time   `json:"dumped_at"`
	UserMap      []UserEntry `json:"user_map"`
	Cardgroups   []CGEntry   `json:"cardgroups"`
	Cards        []CardEntry `json:"cards"`
	UserCardFSRS []FSRSEntry `json:"user_card_fsrs"`
}

type UserEntry struct {
	SourceUUID string `json:"source_uuid"`
	Email      string `json:"email"`
}

type CGEntry struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CardEntry struct {
	ID          string    `json:"id"`
	CardgroupID string    `json:"cardgroup_id"`
	Front       string    `json:"front"`
	Back        string    `json:"back"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FSRSEntry struct {
	UserID        string    `json:"user_id"`
	CardID        string    `json:"card_id"`
	State         int       `json:"state"`
	Due           time.Time `json:"due"`
	Stability     float64   `json:"stability"`
	Difficulty    float64   `json:"difficulty"`
	Reps          int       `json:"reps"`
	Lapses        int       `json:"lapses"`
	LastReview    time.Time `json:"last_review"`
	ScheduledDays int       `json:"scheduled_days"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func main() {
	dumpCmd := flag.NewFlagSet("dump", flag.ExitOnError)
	dumpDBURL := dumpCmd.String("db-url", "", "database connection URL")
	dumpOut := dumpCmd.String("out", "dump.json", "output file path")

	importCmd := flag.NewFlagSet("import", flag.ExitOnError)
	importDBURL := importCmd.String("db-url", "", "database connection URL")
	importIn := importCmd.String("in", "dump.json", "input file path")

	if len(os.Args) < 2 {
		log.Fatal("usage: seed <dump|import> [flags]")
	}

	switch os.Args[1] {
	case "dump":
		if err := dumpCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(eris.ToString(eris.Wrap(err, "seed: parse dump flags"), true))
		}
		if err := runDump(*dumpDBURL, *dumpOut); err != nil {
			log.Fatal(eris.ToString(err, true))
		}
	case "import":
		if err := importCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(eris.ToString(eris.Wrap(err, "seed: parse import flags"), true))
		}
		if err := runImport(*importDBURL, *importIn); err != nil {
			log.Fatal(eris.ToString(err, true))
		}
	default:
		log.Fatalf("unknown subcommand %q", os.Args[1])
	}
}

func runDump(dbURL, outPath string) error {
	if dbURL == "" {
		return eris.New("seed: --db-url is required")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return eris.Wrap(err, "seed: open db")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("seed: db.Close: %v", err)
		}
	}()

	if err := db.Ping(); err != nil {
		return eris.Wrap(err, "seed: ping db")
	}

	var cardgroups []CGEntry
	cgRows, err := db.Query(`SELECT id, owner_id, name, created_at, updated_at FROM public.cardgroups`)
	if err != nil {
		return eris.Wrap(err, "seed: query cardgroups")
	}
	defer cgRows.Close()
	for cgRows.Next() {
		var cg CGEntry
		if err := cgRows.Scan(&cg.ID, &cg.OwnerID, &cg.Name, &cg.CreatedAt, &cg.UpdatedAt); err != nil {
			return eris.Wrap(err, "seed: scan cardgroup row")
		}
		cardgroups = append(cardgroups, cg)
	}
	if err := cgRows.Err(); err != nil {
		return eris.Wrap(err, "seed: iterate cardgroup rows")
	}
	cgRows.Close()

	var cards []CardEntry
	cardRows, err := db.Query(`SELECT id, cardgroup_id, front, back, created_at, updated_at FROM public.cards`)
	if err != nil {
		return eris.Wrap(err, "seed: query cards")
	}
	defer cardRows.Close()
	for cardRows.Next() {
		var c CardEntry
		if err := cardRows.Scan(&c.ID, &c.CardgroupID, &c.Front, &c.Back, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return eris.Wrap(err, "seed: scan card row")
		}
		cards = append(cards, c)
	}
	if err := cardRows.Err(); err != nil {
		return eris.Wrap(err, "seed: iterate card rows")
	}
	cardRows.Close()

	var fsrsEntries []FSRSEntry
	fsrsRows, err := db.Query(`SELECT user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, scheduled_days, created_at, updated_at FROM public.user_card_fsrs`)
	if err != nil {
		return eris.Wrap(err, "seed: query user_card_fsrs")
	}
	defer fsrsRows.Close()
	for fsrsRows.Next() {
		var f FSRSEntry
		if err := fsrsRows.Scan(&f.UserID, &f.CardID, &f.State, &f.Due, &f.Stability, &f.Difficulty, &f.Reps, &f.Lapses, &f.LastReview, &f.ScheduledDays, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return eris.Wrap(err, "seed: scan user_card_fsrs row")
		}
		fsrsEntries = append(fsrsEntries, f)
	}
	if err := fsrsRows.Err(); err != nil {
		return eris.Wrap(err, "seed: iterate user_card_fsrs rows")
	}
	fsrsRows.Close()

	uuidSet := make(map[string]struct{})
	for _, cg := range cardgroups {
		uuidSet[cg.OwnerID] = struct{}{}
	}
	for _, f := range fsrsEntries {
		uuidSet[f.UserID] = struct{}{}
	}

	var userMap []UserEntry
	for uuid := range uuidSet {
		var email string
		err := db.QueryRow(`SELECT email FROM auth.users WHERE id = $1`, uuid).Scan(&email)
		if err == sql.ErrNoRows {
			log.Printf("seed: no auth.users row for uuid %s, skipping", uuid)
			continue
		}
		if err != nil {
			return eris.Wrapf(err, "seed: query auth.users for uuid %s", uuid)
		}
		userMap = append(userMap, UserEntry{SourceUUID: uuid, Email: email})
	}

	df := DumpFile{
		Version:      1,
		DumpedAt:     time.Now().UTC(),
		UserMap:      userMap,
		Cardgroups:   cardgroups,
		Cards:        cards,
		UserCardFSRS: fsrsEntries,
	}

	data, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return eris.Wrap(err, "seed: marshal dump file")
	}

	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return eris.Wrapf(err, "seed: write dump file %s", outPath)
	}

	fmt.Printf("dump complete: %d cardgroups, %d cards, %d fsrs rows, %d users -> %s\n",
		len(cardgroups), len(cards), len(fsrsEntries), len(userMap), outPath)
	return nil
}

func runImport(dbURL, inPath string) (retErr error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return eris.Wrapf(err, "seed: read dump file %s", inPath)
	}

	var df DumpFile
	if err := json.Unmarshal(data, &df); err != nil {
		return eris.Wrap(err, "seed: unmarshal dump file")
	}

	if df.Version != 1 {
		return eris.Errorf("seed: unsupported dump version %d (expected 1)", df.Version)
	}

	if dbURL == "" {
		return eris.New("seed: --db-url is required")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return eris.Wrap(err, "seed: open db")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("seed: db.Close: %v", err)
		}
	}()

	if err := db.Ping(); err != nil {
		return eris.Wrap(err, "seed: ping db")
	}

	uuidMap := make(map[string]string)
	skipped := make(map[string]struct{})
	skippedUserCount := 0

	for _, u := range df.UserMap {
		var targetID string
		err := db.QueryRow(`SELECT id FROM auth.users WHERE email = $1`, u.Email).Scan(&targetID)
		if err == sql.ErrNoRows {
			log.Printf("seed: user not found for email <redacted> (source uuid %s), skipping", u.SourceUUID)
			skipped[u.SourceUUID] = struct{}{}
			skippedUserCount++
			continue
		}
		if err != nil {
			return eris.Wrapf(err, "seed: query auth.users for uuid %s", u.SourceUUID)
		}
		uuidMap[u.SourceUUID] = targetID
	}

	skippedCGs := make(map[string]struct{})
	for _, cg := range df.Cardgroups {
		if _, skip := skipped[cg.OwnerID]; skip {
			skippedCGs[cg.ID] = struct{}{}
		}
	}

	skippedCards := make(map[string]struct{})
	for _, c := range df.Cards {
		if _, skip := skippedCGs[c.CardgroupID]; skip {
			skippedCards[c.ID] = struct{}{}
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return eris.Wrap(err, "seed: begin tx")
	}
	defer func() {
		if retErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("seed: tx.Rollback failed: %v (original error: %v)", rbErr, retErr)
			}
		}
	}()

	cgInserted := 0
	for _, cg := range df.Cardgroups {
		if _, skip := skippedCGs[cg.ID]; skip {
			continue
		}
		targetOwnerID, ok := uuidMap[cg.OwnerID]
		if !ok {
			log.Printf("seed: cardgroup %s has owner_id %s not in uuidMap; skipping", cg.ID, cg.OwnerID)
			skippedCGs[cg.ID] = struct{}{}
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO public.cardgroups (id, owner_id, name, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE
			  SET owner_id   = EXCLUDED.owner_id,
			      name       = EXCLUDED.name`,
			cg.ID, targetOwnerID, cg.Name, cg.CreatedAt,
		)
		if err != nil {
			return eris.Wrapf(err, "seed: upsert cardgroup %s", cg.ID)
		}
		cgInserted++
	}

	cardInserted := 0
	for _, c := range df.Cards {
		if _, skip := skippedCards[c.ID]; skip {
			continue
		}
		if _, skip := skippedCGs[c.CardgroupID]; skip {
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO public.cards (id, cardgroup_id, front, back, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE
			  SET cardgroup_id = EXCLUDED.cardgroup_id,
			      front        = EXCLUDED.front,
			      back         = EXCLUDED.back`,
			c.ID, c.CardgroupID, c.Front, c.Back, c.CreatedAt,
		)
		if err != nil {
			return eris.Wrapf(err, "seed: upsert card %s", c.ID)
		}
		cardInserted++
	}

	fsrsInserted := 0
	for _, f := range df.UserCardFSRS {
		if _, skip := skipped[f.UserID]; skip {
			continue
		}
		if _, skip := skippedCards[f.CardID]; skip {
			continue
		}
		targetUserID, ok := uuidMap[f.UserID]
		if !ok {
			log.Printf("seed: fsrs entry user_id %s not in uuidMap; skipping", f.UserID)
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO public.user_card_fsrs
			  (user_id, card_id, state, due, stability, difficulty, reps, lapses,
			   last_review, scheduled_days, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (user_id, card_id) DO UPDATE
			  SET state          = EXCLUDED.state,
			      due            = EXCLUDED.due,
			      stability      = EXCLUDED.stability,
			      difficulty     = EXCLUDED.difficulty,
			      reps           = EXCLUDED.reps,
			      lapses         = EXCLUDED.lapses,
			      last_review    = EXCLUDED.last_review,
			      scheduled_days = EXCLUDED.scheduled_days`,
			targetUserID, f.CardID, f.State, f.Due, f.Stability, f.Difficulty,
			f.Reps, f.Lapses, f.LastReview, f.ScheduledDays, f.CreatedAt,
		)
		if err != nil {
			return eris.Wrapf(err, "seed: upsert user_card_fsrs user=%s card=%s", f.UserID, f.CardID)
		}
		fsrsInserted++
	}

	if err = tx.Commit(); err != nil {
		return eris.Wrap(err, "seed: commit transaction")
	}

	fmt.Printf("import complete: %d cardgroups, %d cards, %d fsrs rows inserted/updated\n",
		cgInserted, cardInserted, fsrsInserted)
	if skippedUserCount > 0 {
		fmt.Printf("skipped %d users not found in target db\n", skippedUserCount)
	}
	return nil
}

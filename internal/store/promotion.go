package store

import (
	"database/sql"
	"fmt"
)

// Promotion is the event of an athlete reaching a rank on a date (CONTEXT.md).
// Those three are what CreatePromotion writes; ID is the row's own identity.
type Promotion struct {
	ID         int64
	AthleteID  int64
	RankID     int64
	PromotedOn string
}

// PromotionRow is one line of a graduation history: the Promotion plus what a view
// needs to render it, the same way RosterRow relates to Athlete — see RosterRow for
// why these particular fields travel with a row. They are denormalised by
// ListPromotions and have no meaning on write.
type PromotionRow struct {
	Promotion
	Rank Rank
}

// CreatePromotion records a promotion and returns its id. The derived current rank
// falls out of the dates (ADR-0001).
//
// Unlike an athlete's dates, this one is required (promoted_on is NOT NULL), so a
// blank date is malformed rather than absent.
func CreatePromotion(db *sql.DB, p Promotion) (int64, error) {
	if err := checkDate("promoted_on", p.PromotedOn); err != nil {
		return 0, err
	}
	res, err := db.Exec(
		`INSERT INTO promotions (athlete_id, rank_id, promoted_on) VALUES (?, ?, ?)`,
		p.AthleteID, p.RankID, p.PromotedOn,
	)
	if err != nil {
		return 0, fmt.Errorf("insert promotion: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for promotion: %w", err)
	}
	return id, nil
}

// ListPromotions returns an athlete's full graduation history, most recent first,
// joined with the rank and grading-system names for display. Ties on the same
// date fall back to insertion order (id) so the ordering is deterministic and
// matches CurrentRank's tie-break.
func ListPromotions(db *sql.DB, athleteID int64) ([]PromotionRow, error) {
	rows, err := db.Query(`
		SELECT p.id, p.athlete_id, p.rank_id, p.promoted_on, r.name, g.name, g.slug,
		       r.rank_group, r.degree
		FROM promotions p
		JOIN ranks r ON r.id = p.rank_id
		JOIN grading_systems g ON g.id = r.grading_system_id
		WHERE p.athlete_id = ?
		ORDER BY p.promoted_on DESC, p.id DESC`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("list promotions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var promotions []PromotionRow
	for rows.Next() {
		var (
			p        PromotionRow
			promoted sql.NullTime
		)
		if err := rows.Scan(&p.ID, &p.AthleteID, &p.RankID, &promoted, &p.Rank.Name, &p.Rank.System.Name, &p.Rank.System.Slug, &p.Rank.Group, &p.Rank.Degree); err != nil {
			return nil, fmt.Errorf("scan promotion row: %w", err)
		}
		p.Rank.ID = p.RankID
		p.PromotedOn = formatDate(promoted)
		promotions = append(promotions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotions: %w", err)
	}
	return promotions, nil
}

// CurrentRank derives the athlete's current rank from their promotions: the most
// recent by date, cross-system (ADR-0001). It is a pure function over whatever
// promotions it is given, so ordering of the input does not matter. On a tied
// date the more recently recorded promotion (higher id) wins. Returns ok=false
// when the athlete has no promotions. ISO yyyy-mm-dd dates compare lexically, so
// string comparison is a correct date comparison.
func CurrentRank(promotions []PromotionRow) (PromotionRow, bool) {
	var best PromotionRow
	found := false
	for _, p := range promotions {
		switch {
		case !found,
			p.PromotedOn > best.PromotedOn,
			p.PromotedOn == best.PromotedOn && p.ID > best.ID:
			best = p
			found = true
		}
	}
	return best, found
}

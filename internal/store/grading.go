package store

import (
	"database/sql"
	"fmt"
)

// Rank is one named position within a grading system, carrying the descriptive
// group/degree metadata (ADR-0001) for display. It is what a promotion targets.
// Name is English and internal: what a trainer reads is rankview.RankName, composed
// from Group and Degree (ADR-0009).
//
// System is the grading system the rank sits in. It repeats what a parent
// GradingSystem already says, so that a Rank is self-describing wherever it is
// read; like the rest of a read model, it has no meaning on write.
type Rank struct {
	ID     int64
	Name   string
	Group  string
	Degree int
	System System
}

// IsZero reports whether r is no rank at all, as an ungraded athlete's is.
//
// It asks the id because that is the field the roster's LEFT JOIN leaves zero, and
// so the only one that decides. The system slug only correlates: it is empty on a
// system that never passed through ensureGradingSystem, whose ranks are still real.
func (r Rank) IsZero() bool {
	return r.ID == 0
}

// System is a grading system's own two facts: Slug is its identity (ADR-0006) and
// Name only its display label, which the view localizes off the slug (ADR-0009).
type System struct {
	Name string
	Slug string
}

// GradingSystem is an ordered set of ranks for one discipline-and-cohort
// (CONTEXT.md). Ranks are held in display order (sort_order).
type GradingSystem struct {
	ID int64
	System
	Ranks []Rank
}

// ListGradingSystems returns every seeded grading system with its ranks in
// display order, ready to populate the promotion form (system → rank). Systems
// are ordered by sort_order (kids before adult), name breaking ties; ranks
// within a system by sort_order.
func ListGradingSystems(db *sql.DB) ([]GradingSystem, error) {
	rows, err := db.Query(`
		SELECT g.id, g.name, g.slug, r.id, r.name, r.rank_group, r.degree
		FROM grading_systems g
		JOIN ranks r ON r.grading_system_id = g.id
		ORDER BY g.sort_order, g.name, r.sort_order`)
	if err != nil {
		return nil, fmt.Errorf("list grading systems: %w", err)
	}
	defer rows.Close()

	// Rows arrive grouped by system (ORDER BY g.name), so a new system id starts
	// a new group and ranks append to the current one.
	var systems []GradingSystem
	for rows.Next() {
		var (
			gsID int64
			rank Rank
		)
		if err := rows.Scan(&gsID, &rank.System.Name, &rank.System.Slug, &rank.ID, &rank.Name, &rank.Group, &rank.Degree); err != nil {
			return nil, fmt.Errorf("scan grading system row: %w", err)
		}
		if len(systems) == 0 || systems[len(systems)-1].ID != gsID {
			systems = append(systems, GradingSystem{ID: gsID, System: rank.System})
		}
		cur := &systems[len(systems)-1]
		cur.Ranks = append(cur.Ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grading systems: %w", err)
	}
	return systems, nil
}

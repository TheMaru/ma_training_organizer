package store

import (
	"database/sql"
	"fmt"
)

// Rank is one named position within a grading system, carrying the descriptive
// group/degree metadata (ADR-0001) for display. It is what a promotion targets.
// Name is English and internal: what a trainer reads is composed in the view from
// Group and Degree (ADR-0009).
type Rank struct {
	ID     int64
	Name   string
	Group  string
	Degree int
}

// GradingSystem is an ordered set of ranks for one discipline-and-cohort
// (CONTEXT.md). Ranks are held in display order (sort_order). Slug is the system's
// identity (ADR-0006) and Name only its display label, which the view localizes
// off the slug (ADR-0009).
type GradingSystem struct {
	ID    int64
	Name  string
	Slug  string
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
			gsID           int64
			gsName, gsSlug string
			rank           Rank
		)
		if err := rows.Scan(&gsID, &gsName, &gsSlug, &rank.ID, &rank.Name, &rank.Group, &rank.Degree); err != nil {
			return nil, fmt.Errorf("scan grading system row: %w", err)
		}
		if len(systems) == 0 || systems[len(systems)-1].ID != gsID {
			systems = append(systems, GradingSystem{ID: gsID, Name: gsName, Slug: gsSlug})
		}
		cur := &systems[len(systems)-1]
		cur.Ranks = append(cur.Ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grading systems: %w", err)
	}
	return systems, nil
}

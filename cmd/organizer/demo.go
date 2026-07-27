package main

import (
	"database/sql"
	"fmt"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// demoPromotion is one graduation in the demo dataset, targeting a seeded rank
// by grading-system and rank name (so it stays readable and independent of
// auto-increment ids).
type demoPromotion struct {
	system string
	rank   string
	date   string
}

// demoAthlete is one fixed entry in the demo dataset. Identity is (firstName,
// lastName): seedDemo skips an athlete whose name already exists, which keeps
// the whole seed idempotent, and clearDemo removes exactly these names.
type demoAthlete struct {
	firstName  string
	lastName   string
	birthDate  string
	joinedOn   string
	notes      string
	promotions []demoPromotion
}

// demoAthletes is a small, realistic roster spanning the states worth exercising
// by hand: adults progressing through the adult system, kids in the kids system,
// a cross-system athlete (kids → adult, current rank should be the adult one),
// and a fresh athlete with no promotion yet (blank current rank).
//
// It is also the visual test set for the belt graphic (spec, ADR-0004), which is
// why the twelve are curated rather than merely plausible: between their current
// ranks and their histories they reach every body colour the view can draw, both
// split-belt bars and plain belts, stripe degrees 0 through 4, a split belt that
// also carries stripes, and a white belt with stripes (a friso on a light body).
// The current ranks are picked to spread most of that across the roster itself, so
// the graphic-only rendering can be checked at a glance; the rest — the striped
// white belt among them — shows up in a "Verlauf" table, where each belt sits
// beside its own label. TestSeedDemoCoversTheBeltVisuals keeps this from eroding.
var demoAthletes = []demoAthlete{
	{
		firstName: "Tom", lastName: "Albrecht", birthDate: "2016-03-08", joinedOn: "2025-01-13",
		promotions: []demoPromotion{
			{"BJJ Kids", "White", "2025-04-05"},
			{"BJJ Kids", "White, 3 stripes", "2025-11-21"},
			{"BJJ Kids", "Yellow-White", "2026-05-16"},
		},
	},
	{
		firstName: "Lena", lastName: "Bergmann", birthDate: "1992-04-18", joinedOn: "2023-09-01",
		promotions: []demoPromotion{
			{"BJJ Adult", "White", "2023-10-15"},
			{"BJJ Adult", "White, 2 stripes", "2024-05-01"},
			{"BJJ Adult", "Blue", "2025-02-20"},
			{"BJJ Adult", "Purple", "2026-04-10"},
		},
	},
	{
		firstName: "Ida", lastName: "Brandt", birthDate: "2012-01-19", joinedOn: "2022-10-05",
		promotions: []demoPromotion{
			{"BJJ Kids", "Orange-Black", "2024-05-17"},
			{"BJJ Kids", "Green-White", "2025-01-24"},
			{"BJJ Kids", "Green", "2025-08-15"},
			{"BJJ Kids", "Green-Black", "2026-06-05"},
		},
	},
	{
		firstName: "Jonas", lastName: "Fischer", birthDate: "1988-11-30", joinedOn: "2022-01-10",
		promotions: []demoPromotion{
			{"BJJ Adult", "White", "2022-03-01"},
			{"BJJ Adult", "Blue", "2023-06-12"},
			{"BJJ Adult", "Blue, 2 stripes", "2024-08-01"},
			{"BJJ Adult", "Blue, 4 stripes", "2026-01-20"},
		},
	},
	{
		firstName: "Mia", lastName: "Hoffmann", birthDate: "2014-06-22", joinedOn: "2024-02-01",
		promotions: []demoPromotion{
			{"BJJ Kids", "Grey-White", "2024-05-10"},
			{"BJJ Kids", "Grey-White, 2 stripes", "2025-09-18"},
		},
	},
	{
		firstName: "Elias", lastName: "Keller", birthDate: "2008-03-14", joinedOn: "2019-08-20",
		notes: "Wechsel ins Erwachsenensystem mit 16.",
		// Cross-system: kids first, then adult later — current rank is the adult one.
		promotions: []demoPromotion{
			{"BJJ Kids", "Green", "2022-11-02"},
			{"BJJ Adult", "White", "2024-09-01"},
		},
	},
	{
		firstName: "Hannah", lastName: "Krüger", birthDate: "1985-06-03", joinedOn: "2015-04-01",
		notes: "Langjährige Athletin, führt das Erwachsenentraining mit.",
		promotions: []demoPromotion{
			{"BJJ Adult", "Purple", "2017-05-20"},
			{"BJJ Adult", "Brown", "2020-03-14"},
			{"BJJ Adult", "Brown, 2 stripes", "2022-06-18"},
			{"BJJ Adult", "Black", "2024-11-30"},
		},
	},
	{
		firstName: "Sophie", lastName: "Neumann", birthDate: "1997-07-09", joinedOn: "2026-06-15",
		notes: "Neu, noch keine Graduierung.",
	},
	{
		firstName: "Noah", lastName: "Schäfer", birthDate: "2013-12-01", joinedOn: "2023-03-05",
		promotions: []demoPromotion{
			{"BJJ Kids", "Yellow", "2023-06-01"},
			{"BJJ Kids", "Yellow-Black", "2024-10-12"},
			{"BJJ Kids", "Yellow-Black, 1 stripe", "2025-11-08"},
		},
	},
	{
		firstName: "Liam", lastName: "Vogel", birthDate: "2015-09-25", joinedOn: "2024-09-01",
		promotions: []demoPromotion{
			{"BJJ Kids", "White", "2024-12-06"},
			{"BJJ Kids", "Grey-White", "2025-06-13"},
			{"BJJ Kids", "Grey", "2026-02-27"},
		},
	},
	{
		firstName: "Emma", lastName: "Wagner", birthDate: "1990-02-28", joinedOn: "2024-11-11",
		notes: "Quereinsteigerin, Purple aus dem alten Verein.",
		promotions: []demoPromotion{
			{"BJJ Adult", "Purple", "2025-01-20"},
			{"BJJ Adult", "Purple, 3 stripes", "2025-09-05"},
			{"BJJ Adult", "Brown", "2026-05-30"},
		},
	},
	{
		firstName: "Paul", lastName: "Zimmermann", birthDate: "2011-10-17", joinedOn: "2021-09-01",
		promotions: []demoPromotion{
			{"BJJ Kids", "Grey", "2021-12-10"},
			{"BJJ Kids", "Yellow", "2023-04-14"},
			{"BJJ Kids", "Orange", "2025-02-11"},
			{"BJJ Kids", "Orange, 3 stripes", "2026-03-22"},
		},
	},
}

// seedDemo populates the database with the demo roster for hands-on testing. It
// first seeds the grading systems (so the demo promotions have ranks to target),
// then inserts each demo athlete and their promotions. It is idempotent: an
// athlete whose (firstName, lastName) already exists is skipped entirely, so
// re-running never creates duplicates.
func seedDemo(db *sql.DB) error {
	if err := store.Seed(db); err != nil {
		return err
	}
	ranks, err := rankIndex(db)
	if err != nil {
		return err
	}
	existing, err := athleteNames(db)
	if err != nil {
		return err
	}

	for _, d := range demoAthletes {
		if existing[nameKey(d.firstName, d.lastName)] != 0 {
			continue // already present — keep the seed idempotent
		}
		id, err := store.CreateAthlete(db, store.Athlete{
			FirstName: d.firstName, LastName: d.lastName,
			BirthDate: d.birthDate, JoinedOn: d.joinedOn, Notes: d.notes,
		})
		if err != nil {
			return fmt.Errorf("create demo athlete %s %s: %w", d.firstName, d.lastName, err)
		}
		for _, p := range d.promotions {
			rankID, ok := ranks[p.system][p.rank]
			if !ok {
				return fmt.Errorf("demo promotion references unknown rank %q in %q", p.rank, p.system)
			}
			if _, err := store.CreatePromotion(db, store.Promotion{
				AthleteID: id, RankID: rankID, PromotedOn: p.date,
			}); err != nil {
				return fmt.Errorf("create demo promotion for %s %s: %w", d.firstName, d.lastName, err)
			}
		}
	}
	return nil
}

// clearDemo removes exactly the demo athletes (by name), cascading to their
// promotions. Athletes not in the demo set are left untouched, and the seeded
// grading systems are never removed. It is idempotent: absent demo athletes are
// simply skipped.
func clearDemo(db *sql.DB) error {
	existing, err := athleteNames(db)
	if err != nil {
		return err
	}
	for _, d := range demoAthletes {
		id := existing[nameKey(d.firstName, d.lastName)]
		if id == 0 {
			continue
		}
		if err := store.DeleteAthlete(db, id); err != nil {
			return fmt.Errorf("delete demo athlete %s %s: %w", d.firstName, d.lastName, err)
		}
	}
	return nil
}

// rankIndex builds a system-name → rank-name → rank-id lookup from the seeded
// grading systems, so demo promotions can name their target rank in prose.
func rankIndex(db *sql.DB) (map[string]map[string]int64, error) {
	systems, err := store.ListGradingSystems(db)
	if err != nil {
		return nil, err
	}
	index := make(map[string]map[string]int64, len(systems))
	for _, s := range systems {
		ranks := make(map[string]int64, len(s.Ranks))
		for _, r := range s.Ranks {
			ranks[r.Name] = r.ID
		}
		index[s.Name] = ranks
	}
	return index, nil
}

// athleteNames returns a (firstName, lastName) → id map of all athletes, used
// both to skip existing demo athletes on seed and to find them on clear.
func athleteNames(db *sql.DB) (map[string]int64, error) {
	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		return nil, err
	}
	names := make(map[string]int64, len(athletes))
	for _, a := range athletes {
		names[nameKey(a.FirstName, a.LastName)] = a.ID
	}
	return names, nil
}

// nameKey joins first and last name into a map key. The NUL separator can never
// appear in a real name, so distinct names never collide.
func nameKey(first, last string) string {
	return first + "\x00" + last
}

// cmdSeedDemo wires the seed-demo subcommand: open+migrate the database, then
// populate the demo roster.
func cmdSeedDemo(dbPath string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: organizer seed-demo")
	}
	return withDB(dbPath, func(db *sql.DB) error {
		if err := seedDemo(db); err != nil {
			return err
		}
		fmt.Printf("seeded %d demo athletes\n", len(demoAthletes))
		return nil
	})
}

// cmdClearDemo wires the clear-demo subcommand: remove the demo roster again.
func cmdClearDemo(dbPath string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: organizer clear-demo")
	}
	return withDB(dbPath, func(db *sql.DB) error {
		if err := clearDemo(db); err != nil {
			return err
		}
		fmt.Printf("removed demo athletes\n")
		return nil
	})
}

package web

import (
	"slices"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// rosterPageOf is reached from inside the package because it is unexported, and it
// is unexported because the URLs it builds belong to this package's router. What
// is worth pinning about the roster page is its values — which header is active,
// where each link points, which chips exist — not the markup they are rendered
// into, so these build store.RosterView values directly and touch no database or
// server. Prior art: internal/store/roster_internal_test.go.

func headerLabelled(t *testing.T, page rosterPage, label string) rosterHeader {
	t.Helper()
	for _, h := range page.Headers {
		if h.Label == label {
			return h
		}
	}
	t.Fatalf("no header %q in %+v", label, page.Headers)
	return rosterHeader{}
}

func TestRosterPageDefaultsToFirstNameAscending(t *testing.T) {
	page := rosterPageOf(i18n.German, store.RosterView{Query: store.RosterQuery{Sort: store.RosterSortDefault}})

	// The default column is the active one, so it carries the ascending indicator
	// and its own link toggles to descending.
	want := rosterHeader{
		Label: "Vorname", Href: "/athletes?sort=firstName&dir=desc",
		Active: true, Indicator: "▲", AriaSort: "ascending",
	}
	if got := headerLabelled(t, page, "Vorname"); got != want {
		t.Errorf("Vorname header = %+v, want %+v", got, want)
	}
	want = rosterHeader{Label: "Nachname", Href: "/athletes?sort=lastName&dir=asc", AriaSort: "none"}
	if got := headerLabelled(t, page, "Nachname"); got != want {
		t.Errorf("Nachname header = %+v, want %+v", got, want)
	}
}

func TestRosterPageSortsByRequestedColumn(t *testing.T) {
	page := rosterPageOf(i18n.German, store.RosterView{
		Query: store.RosterQuery{Sort: store.RosterSortLastName, Descending: true},
	})

	want := rosterHeader{
		Label: "Nachname", Href: "/athletes?sort=lastName&dir=asc",
		Active: true, Descending: true, Indicator: "▼", AriaSort: "descending",
	}
	if got := headerLabelled(t, page, "Nachname"); got != want {
		t.Errorf("Nachname header = %+v, want %+v", got, want)
	}
	if headerLabelled(t, page, "Vorname").Active {
		t.Error("want Vorname inactive once Nachname is active")
	}
}

func TestRosterPageOffersEveryColumnInDisplayOrder(t *testing.T) {
	page := rosterPageOf(i18n.German, store.RosterView{Query: store.RosterQuery{Sort: store.RosterSortDefault}})

	want := []string{"Nachname", "Vorname", "Geburtsdatum", "Eintritt", "Aktueller Rang"}
	got := make([]string, len(page.Headers))
	for i, h := range page.Headers {
		got[i] = h.Label
	}
	if !slices.Equal(got, want) {
		t.Errorf("headers = %v, want %v", got, want)
	}
}

var (
	kidsOption     = store.RosterOption{Value: "bjj-kids", Name: "BJJ Kids"}
	adultOption    = store.RosterOption{Value: "bjj-adult", Name: "BJJ Adult"}
	ungradedOption = store.RosterOption{Value: store.RosterFilterUngraded}
)

// mixedOptions are the options of a roster with one athlete in every cell of its
// partition, in the order the store offers them.
func mixedOptions() []store.RosterOption {
	return []store.RosterOption{kidsOption, adultOption, ungradedOption}
}

func filterLabels(filters []rosterFilter) []string {
	labels := make([]string, len(filters))
	for i, f := range filters {
		labels[i] = f.Label
	}
	return labels
}

func TestRosterPageFilterChipsOfferEveryNonEmptyCell(t *testing.T) {
	page := rosterPageOf(i18n.German, store.RosterView{
		Query:   store.RosterQuery{Sort: store.RosterSortDefault},
		Options: mixedOptions(),
	})

	// The unfiltered view is a selection too: Alle is active, not nothing.
	want := []rosterFilter{
		{Label: "Alle", Href: "/athletes", Active: true},
		{Label: "BJJ Kinder", Href: "/athletes?sort=firstName&dir=asc&system=bjj-kids"},
		{Label: "BJJ Erwachsene", Href: "/athletes?sort=firstName&dir=asc&system=bjj-adult"},
		{Label: "Ohne Graduierung", Href: "/athletes?sort=firstName&dir=asc&system=none"},
	}
	if !slices.Equal(page.Filters, want) {
		t.Errorf("filters = %+v, want %+v", page.Filters, want)
	}

	page = rosterPageOf(i18n.German, store.RosterView{
		Query:   store.RosterQuery{Sort: store.RosterSortDefault, Filter: "bjj-kids"},
		Options: mixedOptions(),
	})
	active := []string{}
	for _, f := range page.Filters {
		if f.Active {
			active = append(active, f.Label)
		}
	}
	if !slices.Equal(active, []string{"BJJ Kinder"}) {
		t.Errorf("active filters = %v, want [BJJ Kinder]", active)
	}
	if page.Filters[0].Href != "/athletes" {
		t.Errorf("Alle href = %q, want a bare /athletes", page.Filters[0].Href)
	}
}

func TestRosterPageFilterAndSortCompose(t *testing.T) {
	// Narrowing does not reorder: a chip keeps the sort.
	page := rosterPageOf(i18n.German, store.RosterView{
		Query:   store.RosterQuery{Sort: store.RosterSortLastName, Descending: true},
		Options: mixedOptions(),
	})
	if got, want := page.Filters[1].Href, "/athletes?sort=lastName&dir=desc&system=bjj-kids"; got != want {
		t.Errorf("BJJ Kinder href = %q, want %q", got, want)
	}

	page = rosterPageOf(i18n.German, store.RosterView{
		Query:   store.RosterQuery{Sort: store.RosterSortDefault, Filter: "bjj-kids"},
		Options: mixedOptions(),
	})
	if got, want := headerLabelled(t, page, "Nachname").Href, "/athletes?sort=lastName&dir=asc&system=bjj-kids"; got != want {
		t.Errorf("Nachname href = %q, want %q", got, want)
	}
}

func TestRosterPageHasNoFilterChipsBelowTwoOptions(t *testing.T) {
	// A homogeneous roster has nothing to partition: Alle and the one option would
	// show the same list.
	page := rosterPageOf(i18n.German, store.RosterView{
		Query:   store.RosterQuery{Sort: store.RosterSortDefault},
		Options: []store.RosterOption{kidsOption},
	})
	if len(page.Filters) != 0 {
		t.Errorf("filters = %+v, want none for one option", page.Filters)
	}

	page = rosterPageOf(i18n.German, store.RosterView{
		Query:   store.RosterQuery{Sort: store.RosterSortDefault},
		Options: []store.RosterOption{kidsOption, ungradedOption},
	})
	if got, want := filterLabels(page.Filters), []string{"Alle", "BJJ Kinder", "Ohne Graduierung"}; !slices.Equal(got, want) {
		t.Errorf("filters = %v, want %v", got, want)
	}
}

func TestRosterPageLinksCarryTheQuery(t *testing.T) {
	rows := []store.RosterRow{{Athlete: store.Athlete{ID: 7, FirstName: "Ada", LastName: "Lovelace"}}}
	cases := []struct {
		name  string
		query store.RosterQuery
		want  string
	}{
		{"default", store.RosterQuery{Sort: store.RosterSortDefault}, ""},
		{"sorted", store.RosterQuery{Sort: store.RosterSortLastName, Descending: true}, "?sort=lastName&dir=desc"},
		{"filtered", store.RosterQuery{Sort: store.RosterSortDefault, Filter: "bjj-kids"}, "?sort=firstName&dir=asc&system=bjj-kids"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := rosterPageOf(i18n.German, store.RosterView{Query: c.query, Rows: rows})
			line := page.Athletes[0]
			for _, link := range []struct{ name, got, want string }{
				{"Href", line.Href, "/athletes/7" + c.want},
				{"DeleteAction", line.DeleteAction, "/athletes/7/delete" + c.want},
				{"NewHref", page.NewHref, "/athletes/new" + c.want},
				{"Return", page.Return, "/athletes" + c.want},
			} {
				if link.got != link.want {
					t.Errorf("%s = %q, want %q", link.name, link.got, link.want)
				}
			}
		})
	}
}

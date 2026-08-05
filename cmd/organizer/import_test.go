package main

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// importCSV runs the import core over a literal CSV body, failing the test on an
// unexpected error. The leading newline of a raw-string fixture is trimmed so the
// fixtures can start on their own line and still have the header as line 1.
func importCSV(t *testing.T, db *sql.DB, body string) importReport {
	t.Helper()
	report, err := importAthletes(db, strings.NewReader(strings.TrimPrefix(body, "\n")))
	if err != nil {
		t.Fatalf("importAthletes: %v", err)
	}
	return report
}

// importCSVErr runs the import core expecting it to abort, and returns the German
// abort report.
func importCSVErr(t *testing.T, db *sql.DB, body string) string {
	t.Helper()
	report, err := importAthletes(db, strings.NewReader(strings.TrimPrefix(body, "\n")))
	if err == nil {
		t.Fatalf("importAthletes succeeded (%+v), want an abort", report)
	}
	return err.Error()
}

// athleteByName looks up one imported athlete by name, failing if absent.
func athleteByName(t *testing.T, db *sql.DB, first, last string) store.Athlete {
	t.Helper()
	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	for _, a := range athletes {
		if a.FirstName == first && a.LastName == last {
			return a
		}
	}
	t.Fatalf("athlete %s %s not imported (have %+v)", first, last, athletes)
	return store.Athlete{}
}

// countAthletes returns how many athletes are on the roster.
func countAthletes(t *testing.T, db *sql.DB) int {
	t.Helper()
	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	return len(athletes)
}

func TestImportAthletesInsertsEveryRow(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSV(t, db, `
Vorname,Nachname,Geburtsdatum,Beitritt,Notizen
Lena,Bergmann,1992-04-18,2023-09-01,Wettkampfteam
Jonas,Fischer,1988-11-30,2022-01-10,
`)

	if report.imported != 2 || len(report.skipped) != 0 {
		t.Errorf("report = %+v, want 2 imported and nothing skipped", report)
	}
	lena := athleteByName(t, db, "Lena", "Bergmann")
	if lena.BirthDate != "1992-04-18" || lena.JoinedOn != "2023-09-01" || lena.Notes != "Wettkampfteam" {
		t.Errorf("Lena = %+v, want the CSV's dates and notes", lena)
	}
	jonas := athleteByName(t, db, "Jonas", "Fischer")
	if jonas.Notes != "" {
		t.Errorf("Jonas notes = %q, want empty", jonas.Notes)
	}
}

func TestImportAthletesIsIdempotent(t *testing.T) {
	db := storetest.NewDB(t)
	const csv = `
Vorname,Nachname
Lena,Bergmann
Jonas,Fischer
`

	importCSV(t, db, csv)
	report := importCSV(t, db, csv)

	if report.imported != 0 {
		t.Errorf("re-run imported %d, want 0", report.imported)
	}
	if want := []string{"Lena Bergmann", "Jonas Fischer"}; !slices.Equal(report.skipped, want) {
		t.Errorf("skipped = %v, want %v", report.skipped, want)
	}
	if n := countAthletes(t, db); n != 2 {
		t.Errorf("athletes = %d, want 2 (a re-run must not duplicate)", n)
	}
}

// A hand-edit in the web UI must survive a re-import: an existing athlete is
// skipped, never overwritten from the (possibly stale) CSV.
func TestImportAthletesSkipsRatherThanUpdates(t *testing.T) {
	db := storetest.NewDB(t)
	if _, err := store.CreateAthlete(db, store.Athlete{
		FirstName: "Lena", LastName: "Bergmann", Notes: "im Web gepflegt",
	}); err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	report := importCSV(t, db, `
Vorname,Nachname,Notizen
Lena,Bergmann,aus der alten Tabelle
`)

	if report.imported != 0 || len(report.skipped) != 1 {
		t.Errorf("report = %+v, want 0 imported and 1 skipped", report)
	}
	if got := athleteByName(t, db, "Lena", "Bergmann").Notes; got != "im Web gepflegt" {
		t.Errorf("notes = %q, want the hand-edit left untouched", got)
	}
}

// The realistic worst case: a German Excel export — BOM, semicolons, dd.mm.yyyy
// dates, columns in a different order, and an extra column the tool knows
// nothing about.
func TestImportAthletesReadsGermanExcelExport(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSV(t, db, "\ufeffNachname;Mitgliedsnummer;Vorname;Beitritt;Geburtsdatum\r\n"+
		"Bergmann;4711;Lena;01.09.2023;18.04.1992\r\n")

	if report.imported != 1 {
		t.Fatalf("report = %+v, want 1 imported", report)
	}
	lena := athleteByName(t, db, "Lena", "Bergmann")
	if lena.BirthDate != "1992-04-18" || lena.JoinedOn != "2023-09-01" {
		t.Errorf("dates = %q/%q, want them normalised to ISO", lena.BirthDate, lena.JoinedOn)
	}
	// Notizen was not in the file at all.
	if lena.Notes != "" {
		t.Errorf("notes = %q, want empty for a missing optional column", lena.Notes)
	}
}

func TestImportAthletesMatchesHeadersLoosely(t *testing.T) {
	db := storetest.NewDB(t)

	importCSV(t, db, `
 VORNAME , nachname
Lena, Bergmann
`)

	// Header case and padding are ignored, and the values themselves are trimmed.
	athleteByName(t, db, "Lena", "Bergmann")
}

func TestImportAthletesStoresBlankDatesAsUnset(t *testing.T) {
	db := storetest.NewDB(t)

	importCSV(t, db, `
Vorname,Nachname,Geburtsdatum,Beitritt
Lena,Bergmann,,
`)

	lena := athleteByName(t, db, "Lena", "Bergmann")
	if lena.BirthDate != "" || lena.JoinedOn != "" {
		t.Errorf("dates = %q/%q, want both unset", lena.BirthDate, lena.JoinedOn)
	}
}

// One bad row poisons the whole file: every problem is reported together and
// nothing at all is written (all-or-nothing, spec).
func TestImportAthletesWritesNothingWhenAnyRowIsBad(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSVErr(t, db, `
Vorname,Nachname,Geburtsdatum
Lena,Bergmann,1992-04-18
Jonas,,1988-11-30
Mia,Hoffmann,31.02.2014
Lena,Bergmann,1992-04-18
`)

	for _, want := range []string{
		"Import abgebrochen, 3 Fehler:",
		`Zeile 3: Nachname fehlt`,
		`Zeile 4: Geburtsdatum "31.02.2014" ist kein gültiges Datum`,
		`Zeile 5: doppelter Name "Lena Bergmann", schon in Zeile 2`,
	} {
		if !strings.Contains(report, want) {
			t.Errorf("abort report is missing %q; got:\n%s", want, report)
		}
	}
	if n := countAthletes(t, db); n != 0 {
		t.Errorf("athletes = %d, want 0 — a failed import must roll back entirely", n)
	}
}

// A row that is already broken for another reason still anchors its name: were
// it not to, fixing that row would only reveal the duplicate on the next run —
// the row-by-row fix loop all-or-nothing exists to prevent.
func TestImportAthletesSeesDuplicatesBehindOtherErrors(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSVErr(t, db, `
Vorname,Nachname,Geburtsdatum
Lena,Bergmann,32.01.1990
Lena,Bergmann,1992-04-18
`)

	for _, want := range []string{
		"Import abgebrochen, 2 Fehler:",
		`Zeile 2: Geburtsdatum "32.01.1990" ist kein gültiges Datum`,
		`Zeile 3: doppelter Name "Lena Bergmann", schon in Zeile 2`,
	} {
		if !strings.Contains(report, want) {
			t.Errorf("abort report is missing %q; got:\n%s", want, report)
		}
	}
}

// Two rows both missing a Vorname are two missing-name errors, not a duplicate:
// a blank name is no identity to collide on.
func TestImportAthletesDoesNotPairUpNamelessRows(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSVErr(t, db, `
Vorname,Nachname
,Bergmann
,Bergmann
`)

	if strings.Contains(report, "doppelter Name") {
		t.Errorf("abort report = %q, want two missing-Vorname errors instead", report)
	}
	if !strings.Contains(report, "Import abgebrochen, 2 Fehler:") {
		t.Errorf("abort report = %q, want one error per nameless row", report)
	}
}

func TestImportAthletesRejectsBlankFirstName(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSVErr(t, db, `
Vorname,Nachname
   ,Bergmann
`)

	if !strings.Contains(report, "Zeile 2: Vorname fehlt") {
		t.Errorf("abort report = %q, want a blank-Vorname error on line 2", report)
	}
}

func TestImportAthletesRejectsAnUnparseableJoinDate(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSVErr(t, db, `
Vorname,Nachname,Beitritt
Lena,Bergmann,September 2023
`)

	if !strings.Contains(report, `Zeile 2: Beitritt "September 2023" ist kein gültiges Datum`) {
		t.Errorf("abort report = %q, want a bad-Beitritt error", report)
	}
}

// A missing required column is fatal before any row is looked at, so the trainer
// gets one clear message instead of a per-row error avalanche.
func TestImportAthletesRequiresTheNameColumns(t *testing.T) {
	db := storetest.NewDB(t)

	report := importCSVErr(t, db, `
Nachname,Geburtsdatum
Bergmann,1992-04-18
`)

	if !strings.Contains(report, `Pflichtspalte "Vorname" fehlt`) {
		t.Errorf("abort report = %q, want a missing-Vorname-column error", report)
	}
	if strings.Contains(report, "Zeile") {
		t.Errorf("abort report = %q, want no per-row errors — rows are never read", report)
	}
}

func TestImportAthletesRejectsAFileWithoutAHeader(t *testing.T) {
	db := storetest.NewDB(t)

	if report := importCSVErr(t, db, ""); !strings.Contains(report, "Kopfzeile") {
		t.Errorf("abort report = %q, want a missing-header message", report)
	}
}

func TestImportAthletesReportsAsAnImportError(t *testing.T) {
	db := storetest.NewDB(t)

	_, err := importAthletes(db, strings.NewReader("Vorname,Nachname\n,\n"))

	var ie *importError
	if !errors.As(err, &ie) {
		t.Fatalf("err = %v (%T), want an *importError the CLI can report", err, err)
	}
	if len(ie.problems) != 2 {
		t.Errorf("problems = %+v, want one per missing name", ie.problems)
	}
}

func TestImportReportReadsAsGerman(t *testing.T) {
	tests := []struct {
		name   string
		report importReport
		want   string
	}{
		{
			name:   "nothing skipped",
			report: importReport{imported: 3},
			want:   "3 Athleten importiert, 0 übersprungen.",
		},
		{
			name:   "singular",
			report: importReport{imported: 1},
			want:   "1 Athlet importiert, 0 übersprungen.",
		},
		{
			name:   "skipped names are listed",
			report: importReport{imported: 0, skipped: []string{"Lena Bergmann", "Jonas Fischer"}},
			want: "0 Athleten importiert, 2 übersprungen (bereits vorhanden):\n" +
				"  Lena Bergmann\n  Jonas Fischer",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.report.String(); got != tc.want {
				t.Errorf("report =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

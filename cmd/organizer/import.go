package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// The CSV column names, German like the rest of the domain data (spec), spelled
// the way a header is expected to read — matching is case-insensitive, so these
// double as the display names in error messages. Vorname and Nachname are
// required; the other three are optional and default to empty.
const (
	colFirstName = "Vorname"
	colLastName  = "Nachname"
	colBirthDate = "Geburtsdatum"
	colJoinedOn  = "Beitritt"
	colNotes     = "Notizen"
)

// isoDate is the layout the store keeps dates in; germanDate is the form a
// German spreadsheet exports. Both are accepted on input and normalised to ISO.
const (
	isoDate    = "2006-01-02"
	germanDate = "02.01.2006"
)

// importReport summarises one successful import for the trainer: how many
// athletes were created, and which rows were skipped because that name is
// already on the roster.
type importReport struct {
	imported int
	skipped  []string // "Vorname Nachname", in file order
}

// String renders the German success report printed by the CLI.
func (r importReport) String() string {
	head := fmt.Sprintf("%s importiert, %d übersprungen", countAthletesDE(r.imported), len(r.skipped))
	if len(r.skipped) == 0 {
		return head + "."
	}
	var b strings.Builder
	b.WriteString(head + " (bereits vorhanden):")
	for _, name := range r.skipped {
		b.WriteString("\n  " + name)
	}
	return b.String()
}

// countAthletesDE renders a count with the right German noun form.
func countAthletesDE(n int) string {
	if n == 1 {
		return "1 Athlet"
	}
	return fmt.Sprintf("%d Athleten", n)
}

// importProblem is one reason the file was rejected. line is the 1-based line in
// the CSV, or 0 for a problem with the file as a whole (a missing column).
type importProblem struct {
	line int
	msg  string
}

// String renders one problem as a line of the abort report.
func (p importProblem) String() string {
	if p.line == 0 {
		return p.msg
	}
	return fmt.Sprintf("Zeile %d: %s", p.line, p.msg)
}

// importError collects every problem found in one file. The import is
// all-or-nothing (spec): the trainer sees all of them at once and nothing is
// written, so a broken file is fixed in a single pass rather than row by row.
type importError struct {
	problems []importProblem
}

// Error renders the German abort report the CLI prints verbatim.
func (e *importError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Import abgebrochen, %d Fehler:", len(e.problems))
	for _, p := range e.problems {
		b.WriteString("\n  " + p.String())
	}
	return b.String()
}

// importAthletes reads athlete master data from a CSV file and adds it to the
// roster. It is the testable core behind the import-athletes subcommand.
//
// The whole file is parsed and validated before anything is written: on any
// problem it returns an *importError listing all of them and leaves the database
// untouched. Athletes already on the roster (matched on name, like seed-demo)
// are skipped rather than updated — the web UI is the editing surface, and a
// skip can never clobber a hand-edit — which makes re-running a file safe.
func importAthletes(db *sql.DB, r io.Reader) (importReport, error) {
	rows, err := readCSV(r)
	if err != nil {
		return importReport{}, err
	}

	existing, err := athleteNames(db)
	if err != nil {
		return importReport{}, err
	}

	var (
		problems []importProblem
		fresh    []store.Athlete
		report   importReport
		seen     = make(map[string]int, len(rows)) // name → line it first appeared on
	)
	for _, row := range rows {
		// Duplicates are judged on the raw names, before the row's own validation:
		// a row rejected for some other reason still has to anchor its name, or
		// fixing that row would merely surface the duplicate on the next run —
		// exactly the row-by-row fix loop all-or-nothing exists to avoid. Rows
		// missing a name sit this out; their missing-name error already covers them.
		if row.firstName != "" && row.lastName != "" {
			key := nameKey(row.firstName, row.lastName)
			if first, dup := seen[key]; dup {
				problems = append(problems, importProblem{row.line, fmt.Sprintf(
					"doppelter Name %q, schon in Zeile %d", row.name(), first)})
				continue
			}
			seen[key] = row.line
		}

		a, rowProblems := row.athlete()
		if len(rowProblems) > 0 {
			problems = append(problems, rowProblems...)
			continue
		}
		if existing[nameKey(a.FirstName, a.LastName)] != 0 {
			report.skipped = append(report.skipped, row.name())
			continue
		}
		fresh = append(fresh, a)
	}
	if len(problems) > 0 {
		return importReport{}, &importError{problems}
	}

	if err := store.CreateAthletes(db, fresh); err != nil {
		return importReport{}, err
	}
	report.imported = len(fresh)
	return report, nil
}

// csvRow is one data row, already resolved against the header: the fields the
// importer cares about, plus the line they came from for error messages.
type csvRow struct {
	line                                            int
	firstName, lastName, birthDate, joinedOn, notes string
}

// athlete validates the row and turns it into an Athlete. Validation is
// deliberately no stricter than the web form (spec): the names must be present,
// the dates must parse, and nothing else is second-guessed.
func (row csvRow) athlete() (store.Athlete, []importProblem) {
	var problems []importProblem
	if row.firstName == "" {
		problems = append(problems, importProblem{row.line, colFirstName + " fehlt"})
	}
	if row.lastName == "" {
		problems = append(problems, importProblem{row.line, colLastName + " fehlt"})
	}
	birth, err := parseImportDate(row.birthDate)
	if err != nil {
		problems = append(problems, importProblem{row.line, fmt.Sprintf("%s %v", colBirthDate, err)})
	}
	joined, err := parseImportDate(row.joinedOn)
	if err != nil {
		problems = append(problems, importProblem{row.line, fmt.Sprintf("%s %v", colJoinedOn, err)})
	}
	if len(problems) > 0 {
		return store.Athlete{}, problems
	}
	return store.Athlete{
		FirstName: row.firstName, LastName: row.lastName,
		BirthDate: birth, JoinedOn: joined, Notes: row.notes,
	}, nil
}

// name renders the row's athlete for the report and for duplicate messages.
func (row csvRow) name() string {
	return row.firstName + " " + row.lastName
}

// parseImportDate normalises one date cell to the ISO form the store expects.
// Empty stays empty (the column is optional and lands as SQL NULL).
func parseImportDate(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{isoDate, germanDate} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(isoDate), nil
		}
	}
	return "", fmt.Errorf("%q ist kein gültiges Datum (JJJJ-MM-TT oder TT.MM.JJJJ)", s)
}

// readCSV parses the file into rows resolved against its header. Everything that
// varies between real-world exports is absorbed here — a UTF-8 BOM, a `,` or `;`
// delimiter, column order, unknown extra columns, missing optional columns — so
// the caller only ever sees the five fields it knows.
//
// A missing required column aborts before a single row is read, which spares the
// trainer a per-row error avalanche for one wrong header.
func readCSV(r io.Reader) ([]csvRow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))

	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = sniffDelimiter(data)
	cr.FieldsPerRecord = -1 // ragged rows are handled per field, not fatally
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, &importError{[]importProblem{{0, "Datei enthält keine Kopfzeile"}}}
	}
	if err != nil {
		return nil, parseProblem(err)
	}
	columns, err := indexColumns(header)
	if err != nil {
		return nil, err
	}

	var rows []csvRow
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// A malformed quote leaves the reader out of sync with the file, so
			// report where it broke rather than guessing at the rest.
			return nil, parseProblem(err)
		}
		line, _ := cr.FieldPos(0)
		rows = append(rows, csvRow{
			line:      line,
			firstName: columns.value(record, colFirstName),
			lastName:  columns.value(record, colLastName),
			birthDate: columns.value(record, colBirthDate),
			joinedOn:  columns.value(record, colJoinedOn),
			notes:     columns.value(record, colNotes),
		})
	}
	return rows, nil
}

// columnIndex maps a lower-cased column name to its position in every record.
type columnIndex map[string]int

// value returns the trimmed cell for a column, empty when the column is absent
// from the file or the row stops short of it. The lookup is case-insensitive, so
// callers pass the column's display spelling.
func (c columnIndex) value(record []string, column string) string {
	i, ok := c[strings.ToLower(column)]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

// indexColumns maps the header row, matching names case-insensitively and
// ignoring surrounding space. Unknown columns are simply not indexed, so a club
// export with extra columns still imports.
func indexColumns(header []string) (columnIndex, error) {
	columns := make(columnIndex, len(header))
	for i, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = i
	}
	var missing []importProblem
	for _, required := range []string{colFirstName, colLastName} {
		if _, ok := columns[strings.ToLower(required)]; !ok {
			missing = append(missing, importProblem{0, fmt.Sprintf("Pflichtspalte %q fehlt", required)})
		}
	}
	if len(missing) > 0 {
		return nil, &importError{missing}
	}
	return columns, nil
}

// sniffDelimiter picks the separator from the header line: German Excel writes
// `;`, everything else `,`. Whichever occurs more often in that first line wins,
// with `,` as the tie-break default.
func sniffDelimiter(data []byte) rune {
	headerLine := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		headerLine = data[:i]
	}
	if bytes.Count(headerLine, []byte(";")) > bytes.Count(headerLine, []byte(",")) {
		return ';'
	}
	return ','
}

// parseProblem turns an encoding/csv failure into an importError so structurally
// broken files are reported in the same German form as bad data.
func parseProblem(err error) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return &importError{[]importProblem{{pe.StartLine, fmt.Sprintf("CSV-Format ungültig: %v", pe.Err)}}}
	}
	return fmt.Errorf("read csv: %w", err)
}

// cmdImportAthletes wires the import-athletes subcommand: open+migrate the
// database, read the file, and print the German report. An aborted import prints
// its own report and exits non-zero without a second, English error line.
func cmdImportAthletes(dbPath string, args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return errors.New("usage: organizer import-athletes <file.csv>")
	}
	return withDB(dbPath, func(db *sql.DB) error {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()

		report, err := importAthletes(db, f)
		var ie *importError
		if errors.As(err, &ie) {
			fmt.Println(ie)
			return errAlreadyReported
		}
		if err != nil {
			return err
		}
		fmt.Println(report)
		return nil
	})
}

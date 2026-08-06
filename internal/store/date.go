package store

import (
	"errors"
	"fmt"
	"time"
)

// ISODate is the yyyy-mm-dd layout every date in the store is kept in, matching
// what <input type="date"> submits. It is exported because the callers that parse
// a date before handing it over — the web forms, the CSV import — have to agree
// with the store about what a date is, and a second spelling of the layout is a
// second thing to keep in step.
const ISODate = "2006-01-02"

// ErrMalformedDate is returned by every write handed a date the read path could
// not parse back. Callers match it with errors.Is to tell a correctable input
// apart from a broken database.
//
// The invariant it enforces: the DATE columns take any string, but the driver
// hands DATE values back as time.Time. So an unparseable value is written
// happily, and then fails the scan of every later read — of the whole shared
// roster, not only the row at fault. The write is the last point that still knows
// which value is to blame, which is why it is refused here and not survived
// there.
var ErrMalformedDate = errors.New("store: malformed date")

// checkDate refuses anything but an ISODate string. column names the one at
// fault, because a row can carry more than one date and the caller has to know
// which of them to correct.
func checkDate(column, date string) error {
	if _, err := time.Parse(ISODate, date); err != nil {
		return fmt.Errorf("%w: %s = %q, want %s", ErrMalformedDate, column, date, ISODate)
	}
	return nil
}

// checkOptionalDate is checkDate for the nullable columns, where blank means
// absent rather than malformed (nullIfEmpty stores it as NULL).
func checkOptionalDate(column, date string) error {
	if date == "" {
		return nil
	}
	return checkDate(column, date)
}

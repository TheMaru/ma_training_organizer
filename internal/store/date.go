package store

import (
	"errors"
	"fmt"
	"time"
)

// ISODate is the yyyy-mm-dd layout every date in the store is kept in, matching
// what <input type="date"> submits. It is exported for the callers that render a
// date — a second spelling of the layout is a second thing to keep in step. It is
// not how a caller asks whether a date is acceptable: that question is IsDate.
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

// IsDate reports whether the store would accept date, so a caller can refuse it
// before the write does and answer with a message of its own. The answer is a
// yes/no and not an error on purpose: by ADR-0008 the three callers do not share
// a language, so no sentence from here would be printable by all of them.
//
// This is the only place in the package that parses ISODate.
func IsDate(date string) bool {
	_, err := time.Parse(ISODate, date)
	return err == nil
}

// IsDateOrBlank is IsDate for an optional field, where blank means absent rather
// than malformed.
func IsDateOrBlank(date string) bool {
	return date == "" || IsDate(date)
}

// checkDate refuses anything but an ISODate string. column names the one at
// fault, because a row can carry more than one date and the caller has to know
// which of them to correct.
func checkDate(column, date string) error {
	if !IsDate(date) {
		return fmt.Errorf("%w: %s = %q, want %s", ErrMalformedDate, column, date, ISODate)
	}
	return nil
}

// checkOptionalDate is checkDate for the nullable columns; nullIfEmpty stores a
// blank one as NULL.
func checkOptionalDate(column, date string) error {
	if IsDateOrBlank(date) {
		return nil
	}
	return checkDate(column, date)
}

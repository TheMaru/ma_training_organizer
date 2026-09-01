package web_test

import (
	"database/sql"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer/trainertest"
)

// trainerByID is the read this file counts: store.TrainerByID, matched on the
// clause that tells it from the login's lookup by username.
const trainerByID = "FROM trainers WHERE id"

// trainerByIDReads counts them across the whole test binary. The tests below read
// deltas around one request each, which is what makes a shared counter enough —
// this suite runs its requests sequentially.
var trainerByIDReads atomic.Int64

// One middleware loads the trainer and resolveLocale, requireAuth and the account
// handlers all read it. Counted at the driver rather than asserted by eye, so it
// stays true: the next handler that reaches for the store makes this fail rather
// than making the page slower and nothing else.
func TestAnAuthenticatedRequestReadsTheTrainerOnce(t *testing.T) {
	ts, client := newCountedServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()

	tests := []struct {
		name string
		do   func() *http.Response
	}{
		{"GET /", func() *http.Response { return get(t, ts, client, "/") }},
		{"GET /account/password", func() *http.Response { return get(t, ts, client, "/account/password") }},
		{"POST /account/password", func() *http.Response {
			return post(t, ts, client, "/account/password", url.Values{
				"current": {trainertest.Password},
				"new":     {"a-brand-new-secret"},
				"confirm": {"a-brand-new-secret"},
			})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := trainerByIDReads.Load()
			tt.do().Body.Close()
			if got := trainerByIDReads.Load() - before; got != 1 {
				t.Errorf("%s made %d TrainerByID reads, want 1", tt.name, got)
			}
		})
	}
}

// The other half of the claim: the middleware asks only when the session names an
// id, so the login page costs nothing.
func TestAnAnonymousRequestReadsNoTrainer(t *testing.T) {
	ts, client := newCountedServer(t)

	before := trainerByIDReads.Load()
	get(t, ts, client, "/login").Body.Close()

	if got := trainerByIDReads.Load() - before; got != 0 {
		t.Errorf("GET /login made %d TrainerByID reads, want 0", got)
	}
}

// newCountedServer is the shared fixture with the server's handle swapped for a
// counted one. Everything goes through that handle, the provisioning included, so
// there is one writer on the file.
func newCountedServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	storetest.NewDBOn(t, path)
	counted := openCounted(t, path)
	trainertest.Provision(t, counted, testUsername, spareUsername)

	return startServer(t, counted, time.Hour)
}

// openCounted opens an already-migrated database through the counting driver, on
// the connection string the app itself would have used.
func openCounted(t *testing.T, path string) *sql.DB {
	t.Helper()
	registerCountingDriver(t)
	db, err := sql.Open(countingDriverName, store.DSN(path))
	if err != nil {
		t.Fatalf("open counted database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

const countingDriverName = "sqlite-counting"

var registerOnce sync.Once

// registerCountingDriver registers the driver once for the binary, driver names
// being global.
func registerCountingDriver(t *testing.T) {
	t.Helper()
	registerOnce.Do(func() {
		base, err := sql.Open("sqlite", "")
		if err != nil {
			t.Fatalf("reach the sqlite driver: %v", err)
		}
		defer base.Close()
		sql.Register(countingDriverName, countingDriver{inner: base.Driver()})
	})
}

type countingDriver struct{ inner driver.Driver }

func (d countingDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return countingConn{Conn: conn}, nil
}

// countingConn embeds the driver connection rather than forwarding to it, and
// that is the mechanism: an embedded interface promotes only its own methods, so
// the wrapper does not satisfy driver.QueryerContext even though the real
// connection does. database/sql therefore falls back to Prepare for every
// statement, which is the one place worth counting.
type countingConn struct{ driver.Conn }

func (c countingConn) Prepare(query string) (driver.Stmt, error) {
	if strings.Contains(query, trainerByID) {
		trainerByIDReads.Add(1)
	}
	return c.Conn.Prepare(query)
}

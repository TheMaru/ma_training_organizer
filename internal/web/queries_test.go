package web_test

import (
	"database/sql"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/session"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/web"
)

// trainerByID is the read this file counts: store.TrainerByID, the one an
// authenticated request is meant to make exactly once. Matched on the clause that
// tells it from the login's lookup by username.
const trainerByID = "FROM trainers WHERE id"

// trainerByIDReads counts them across the whole test binary. The tests below read
// deltas around one request each, which is what makes a shared counter enough —
// this suite runs its requests sequentially.
var trainerByIDReads atomic.Int64

// A trainer's row was being read three times per request, once per field wanted:
// the locale middleware took Locale, requireAuth took Deactivated, and the
// password handler took PasswordHash. One middleware loads it now and all three
// read that.
//
// Counted at the driver rather than asserted by eye, so it stays true: the next
// handler that reaches for the store makes this fail rather than making the page
// slower and nothing else.
func TestAnAuthenticatedRequestReadsTheTrainerOnce(t *testing.T) {
	ts, client, _ := newCountedServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	tests := []struct {
		name string
		do   func() *http.Response
	}{
		{"GET /", func() *http.Response { return get(t, ts, client, "/") }},
		{"GET /account/password", func() *http.Response { return get(t, ts, client, "/account/password") }},
		{"POST /account/password", func() *http.Response {
			return post(t, ts, client, "/account/password", url.Values{
				"current": {testPassword},
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

// The login page is served to nobody, so there is no trainer to load. This is the
// other half of the claim: the middleware asks only when the session names an id.
func TestAnAnonymousRequestReadsNoTrainer(t *testing.T) {
	ts, client, _ := newCountedServer(t)

	before := trainerByIDReads.Load()
	get(t, ts, client, "/login").Body.Close()

	if got := trainerByIDReads.Load() - before; got != 0 {
		t.Errorf("GET /login made %d TrainerByID reads, want 0", got)
	}
}

// newCountedServer is newAuthTestServer with the server's database swapped for a
// counted one. The fixture keeps its own handle, so the trainers the helpers
// provision are not counted as the server's reads.
func newCountedServer(t *testing.T) (*httptest.Server, *http.Client, *sql.DB) {
	t.Helper()

	fixture := storetest.NewDB(t)
	addTrainer(t, fixture, testUsername)
	addTrainer(t, fixture, spareUsername)

	counted := openCounted(t, dbPath(t, fixture))
	sessions := session.ForServer(counted, session.Policy{Lifetime: time.Hour, IdleTimeout: time.Hour})
	srv, err := web.NewServer(counted, sessions)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts, newClient(t), fixture
}

// dbPath is the file a *sql.DB is open on, so a second handle can be opened on
// the same database. SQLite-specific, as this whole fixture is.
func dbPath(t *testing.T, db *sql.DB) string {
	t.Helper()
	var path string
	if err := db.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil {
		t.Fatalf("read the database's file path: %v", err)
	}
	return path
}

// openCounted opens an already-migrated database through the counting driver.
func openCounted(t *testing.T, path string) *sql.DB {
	t.Helper()
	registerCountingDriver(t)
	db, err := sql.Open(countingDriverName, "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open counted database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

const countingDriverName = "sqlite-counting"

var registerOnce sync.Once

// registerCountingDriver registers a driver that wraps the real one and counts
// the trainer reads passing through it. Driver names are global, so it happens
// once for the binary.
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

// countingDriver counts what the database was actually asked, rather than what a
// counter planted in the store would say it was asked. It cannot drift from the
// code, and it measures a whole request rather than one function.
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

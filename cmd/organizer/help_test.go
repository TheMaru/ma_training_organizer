package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// All three spellings are the same request, and a request for help is not a
// failure: the listing goes to stdout so `organizer help | less` shows it, and
// the exit code stays 0 so a script asking for usage does not abort.
func TestHelpPrintsTheListingToStdout(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			var err error
			out := captureStdout(t, func() { err = run([]string{arg}) })
			if err != nil {
				t.Fatalf("run(%q) = %v, want nil", arg, err)
			}
			for _, group := range []string{"Accounts:", "Data:", "Development:"} {
				if !strings.Contains(out, group) {
					t.Errorf("listing has no %s group:\n%s", group, out)
				}
			}
		})
	}
}

// An environment the binary refuses to start under is exactly the situation in
// which somebody types `organizer help`, so the listing has to come out before
// the configuration is read rather than after it.
func TestHelpSurvivesABrokenEnvironment(t *testing.T) {
	t.Setenv("ORGANIZER_SESSION_LIFETIME", "not-a-duration")

	var err error
	out := captureStdout(t, func() { err = run([]string{"help"}) })
	if err != nil {
		t.Fatalf("run(help) = %v, want nil", err)
	}
	if !strings.Contains(out, "Accounts:") {
		t.Errorf("no listing printed:\n%s", out)
	}
}

// The listing is rendered from the same table run dispatches from, so a
// subcommand can no longer be reachable and unlisted. What is left to check is
// that the rendering puts each entry where a reader looks for it: on a line of
// its own, under its own group heading, carrying its arguments and what it does.
func TestListingShowsEveryCommandUnderItsGroup(t *testing.T) {
	listing := helpListing()
	for _, c := range commands {
		t.Run(c.Name, func(t *testing.T) {
			line, group := listedUnder(listing, c.Name)
			if line == "" {
				t.Fatalf("no line for %q:\n%s", c.Name, listing)
			}
			if group != c.Group {
				t.Errorf("%q is listed under %q, want %q", c.Name, group, c.Group)
			}
			left, description, _ := strings.Cut(strings.TrimSpace(line), "  ")
			if want := strings.TrimSpace(c.Name + " " + c.Args); left != want {
				t.Errorf("line starts %q, want %q", left, want)
			}
			if got := strings.TrimSpace(description); got != c.Description {
				t.Errorf("line describes it as %q, want %q", got, c.Description)
			}
		})
	}
}

// A second entry under a name already in the table would take a line in the
// listing and never be dispatched: the scan in run stops at the first. Nothing
// else in the package notices.
func TestCommandNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool, len(commands))
	for _, c := range commands {
		if seen[c.Name] {
			t.Errorf("two entries are named %q", c.Name)
		}
		seen[c.Name] = true
	}
}

// The other invariant the table's comment claims: a group gets one heading. The
// table is swapped for one whose groups are interleaved — the state a careless
// insertion leaves behind — because the real table has them in blocks and so
// cannot tell a per-group heading from a per-change one.
func TestEachGroupGetsOneHeading(t *testing.T) {
	swapCommands(t, []command{
		{Name: "first", Group: "Accounts", Description: "one"},
		{Name: "second", Group: "Data", Description: "two"},
		{Name: "third", Group: "Accounts", Description: "three"},
	})

	listing := helpListing()

	if got := strings.Count(listing, "\nAccounts:"); got != 1 {
		t.Errorf("the Accounts heading appears %d times, want 1:\n%s", got, listing)
	}
	// Both entries still under it, or one heading would only mean one was dropped.
	for _, name := range []string{"first", "third"} {
		if _, group := listedUnder(listing, name); group != "Accounts" {
			t.Errorf("%q is listed under %q, want Accounts:\n%s", name, group, listing)
		}
	}
}

// Every entry in the table is reachable through run, driven for real rather than
// looked up in the table a second time. Four arguments is more than any subcommand
// takes, so each is refused by its own usage message before it opens anything —
// which is what makes it safe to drive all ten here, and what identifies the
// subcommand that answered: anything else, an unknown command or a configuration
// that failed to load, produces a different error.
func TestEveryCommandDispatches(t *testing.T) {
	t.Setenv("ORGANIZER_DB_PATH", filepath.Join(t.TempDir(), "never-opened.db"))
	for _, c := range commands {
		t.Run(c.Name, func(t *testing.T) {
			err := run([]string{c.Name, "a", "b", "c", "d"})
			if err == nil {
				t.Fatalf("run(%q with four arguments) = nil, want a usage error", c.Name)
			}
			if want := "usage: organizer " + c.Name; !strings.Contains(err.Error(), want) {
				t.Errorf("run(%q) = %v, want it refused by %q", c.Name, err, want)
			}
		})
	}
}

// The written-out lines and the rendered ones share one description column, which
// is what lets the listing read as a single block. Nothing else holds them
// together: the preamble and the Help group count their spaces by hand.
func TestEveryDescriptionStartsInTheSameColumn(t *testing.T) {
	checked := 0
	for _, text := range strings.Split(helpListing(), "\n") {
		body := strings.TrimRight(text, " ")
		if !strings.HasPrefix(body, "  ") {
			continue // a heading or a blank line
		}
		gap := strings.Index(body[2:], "  ")
		if gap < 0 {
			continue // an entry with no description of its own
		}
		checked++
		if start := len(body) - len(strings.TrimLeft(body[2+gap:], " ")); start != descriptionColumn {
			t.Errorf("description on %q starts at column %d, want %d", text, start, descriptionColumn)
		}
	}
	// The ten entries plus the two lines that are written out, so a skip rule that
	// silently swallowed the hand-aligned half would not leave this passing.
	if want := len(commands) + 2; checked != want {
		t.Errorf("checked %d lines, want %d", checked, want)
	}
}

// What run hands the entry it found: the configured database and the arguments
// after the name. The table is swapped for one none of the real subcommands are
// in, so this stays a test of the dispatch and not of any of them.
func TestRunPassesTheDatabaseAndTheRemainingArguments(t *testing.T) {
	t.Setenv("ORGANIZER_DB_PATH", "configured.db")
	var gotDBPath string
	var gotArgs []string
	swapCommands(t, []command{{
		Name: "ping",
		Run: func(dbPath string, args []string) error {
			gotDBPath, gotArgs = dbPath, args
			return nil
		},
	}})

	if err := run([]string{"ping", "once", "twice"}); err != nil {
		t.Fatalf("run(ping) = %v, want nil", err)
	}
	if gotDBPath != "configured.db" {
		t.Errorf("dbPath = %q, want the configured database", gotDBPath)
	}
	if want := []string{"once", "twice"}; !slices.Equal(gotArgs, want) {
		t.Errorf("args = %v, want %v", gotArgs, want)
	}
}

// A typo gets its own error and not the whole listing, which would push the
// error off the screen — but it does get told where the listing lives.
func TestUnknownCommandPointsAtHelp(t *testing.T) {
	err := run([]string{"promote-everyone"})
	if err == nil {
		t.Fatal("run accepted an unknown command")
	}
	for _, want := range []string{"unknown command: promote-everyone", "organizer help"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// Running with no arguments still starts the server: that path is the deployment
// entry point (ADR-0002) and is guarded by `len(args) > 0`, which this change
// does not touch. It is not driven here because serve blocks until a signal.

func swapCommands(t *testing.T, table []command) {
	t.Helper()
	original := commands
	t.Cleanup(func() { commands = original })
	commands = table
}

// listedUnder returns the listing's line for name together with the heading it
// sits under, or two empty strings if no line starts with name. It anchors on the
// start of a line so a name that happens to appear inside somebody else's
// description does not count as listed.
func listedUnder(listing, name string) (line, group string) {
	heading := ""
	for _, text := range strings.Split(listing, "\n") {
		if before, found := strings.CutSuffix(text, ":"); found && !strings.HasPrefix(text, " ") {
			heading = before
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(text), name+" ") {
			return text, heading
		}
	}
	return "", ""
}

// captureStdout collects everything fn prints to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(out)
}

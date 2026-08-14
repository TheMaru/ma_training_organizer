package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strconv"
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

// The listing is only worth having if it is complete, and completeness is the
// part that rots: the next subcommand is added to the dispatcher and nowhere
// else. So the expectation is read off the dispatcher itself rather than
// written down here a second time.
func TestHelpListsEveryDispatchedCommand(t *testing.T) {
	commands := dispatchedCommands(t)
	if len(commands) < 2 {
		t.Fatalf("found %d commands in the dispatcher, expected the whole switch", len(commands))
	}
	for _, name := range commands {
		if !listsCommand(helpText, name) {
			t.Errorf("subcommand %q is dispatched but has no line in the listing:\n%s", name, helpText)
		}
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

// dispatchedCommands reads the subcommand names out of the switch in run, by
// parsing this package's own source. Asking the source is what makes the
// completeness test above catch a command that was added to the dispatcher and
// forgotten in the listing; a list maintained here would rot in step with it.
func dispatchedCommands(t *testing.T) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		clause, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range clause.List {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			name, err := strconv.Unquote(lit.Value)
			if err != nil {
				continue
			}
			names = append(names, name)
		}
		return true
	})
	return names
}

// listsCommand reports whether the listing gives name a line of its own. It
// anchors on the start of a line rather than searching the whole text, so a
// command whose name happens to appear inside somebody else's description does
// not count as listed.
func listsCommand(listing, name string) bool {
	for _, line := range strings.Split(listing, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), name+" ") {
			return true
		}
	}
	return false
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

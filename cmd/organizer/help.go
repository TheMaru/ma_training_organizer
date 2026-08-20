package main

import (
	"strings"
	"unicode/utf8"
)

// helpListing renders the listing printed for help, -h and --help. It is English
// and holds no catalog lookups: this is the operator's half of the CLI (ADR-0008).
//
// Everything between the preamble and the Help group comes from the command table,
// so the listing is complete by construction rather than by anybody remembering to
// add a line.
func helpListing() string {
	lines := []string{helpPreamble}
	group := ""
	for _, c := range commands {
		if c.Group != group {
			group = c.Group
			lines = append(lines, "", group+":")
		}
		lines = append(lines, listingLine(c))
	}
	return strings.Join(append(lines, "", helpEpilogue), "\n")
}

// listingLine renders one entry the way the listing prints it: the name and its
// argument sketch on the left, the description at descriptionColumn — or, for a
// left side too wide for that column, two spaces after it, because a description
// run together with the arguments would be worse than a ragged one.
func listingLine(c command) string {
	left := "  " + c.Name
	if c.Args != "" {
		left += " " + c.Args
	}
	return left + strings.Repeat(" ", max(descriptionColumn-utf8.RuneCountInString(left), 2)) + c.Description
}

// descriptionColumn is where every description starts. It is a fixed column and
// not the widest entry plus a gap, because the preamble and the Help group are
// written out and have to line up with the rendered entries.
const descriptionColumn = 35

// helpPreamble and helpEpilogue are the two parts no entry in the table produces:
// what the binary is and how to invoke it, and the three spellings of a request
// for this listing. Their descriptions are aligned by hand, which is what
// descriptionColumn exists to keep them agreeing with.
const helpPreamble = `organizer — the roster and the accounts of a martial-arts club.

Usage:
  organizer                        start the web server
  organizer <command> [arguments]`

const helpEpilogue = `Help:
  help, -h, --help                 print this listing`

// isHelpRequest reports whether an argument is one of the three spellings of a
// request for the listing.
func isHelpRequest(arg string) bool {
	switch arg {
	case "help", "-h", "--help":
		return true
	}
	return false
}

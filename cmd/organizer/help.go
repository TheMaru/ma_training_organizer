package main

// helpText is the listing printed for help, -h and --help. It is English and
// holds no catalog lookups: this is the operator's half of the CLI (ADR-0008).
//
// The groups exist because a flat list of every subcommand is a wall. The demo
// pair is listed rather than hidden — whoever eventually finds a hidden command
// finds it without the heading that says what it is for.
const helpText = `organizer — the roster and the accounts of a martial-arts club.

Usage:
  organizer                        start the web server
  organizer <command> [arguments]

Accounts:
  create-trainer <username>        provision a trainer and set their first password
  reset-password <username>        set a new password for a trainer
  revoke-sessions <username>       log a trainer out everywhere
  deactivate-trainer <username>    keep the account, refuse it at login
  reactivate-trainer <username>    let a deactivated trainer back in
  delete-trainer <username>        remove the account for good, after asking
  list-trainers                    show every trainer and who lost access when

Data:
  import-athletes <file.csv>       add athletes to the roster from a CSV file

Development:
  seed-demo                        put the demo athletes on the roster
  clear-demo                       take the demo athletes off again

Help:
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

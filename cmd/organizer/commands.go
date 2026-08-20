package main

// command is one subcommand of the binary, stated once. run dispatches from the
// table below and the help listing is rendered from it, so a subcommand cannot be
// reachable without being listed, nor listed without being reachable.
type command struct {
	// Name is what the operator types after `organizer`.
	Name string
	// Group is the heading the listing files this subcommand under.
	Group string
	// Args is the argument sketch the listing prints, empty for a subcommand that
	// takes none. It is not validation: each Run rejects its own argument shape
	// with its own `usage:` message.
	Args        string
	Description string
	// Run does the work, against the configured database and the arguments after
	// the name.
	Run func(dbPath string, args []string) error
}

// The groups exist because a flat list of every subcommand is a wall. The demo
// pair is listed rather than hidden — whoever eventually finds a hidden command
// finds it without the heading that says what it is for.
const (
	groupAccounts    = "Accounts"
	groupData        = "Data"
	groupDevelopment = "Development"
)

// commands is the binary's subcommands. The order is the order the listing prints
// them in and entries of a group are kept together, which is why this is a slice
// and not a map; dispatch is a linear scan over it.
//
// `help`, `-h` and `--help` are deliberately not here — run answers them itself,
// for the reason stated there — so their line in the listing is written out rather
// than rendered.
var commands = []command{
	{
		Name: "create-trainer", Group: groupAccounts, Args: "<username>",
		Description: "provision a trainer and set their first password",
		Run:         cmdCreateTrainer,
	},
	{
		Name: "reset-password", Group: groupAccounts, Args: "<username>",
		Description: "set a new password for a trainer",
		Run:         cmdResetPassword,
	},
	{
		Name: "revoke-sessions", Group: groupAccounts, Args: "<username>",
		Description: "log a trainer out everywhere",
		Run:         cmdRevokeSessions,
	},
	{
		Name: "deactivate-trainer", Group: groupAccounts, Args: "<username>",
		Description: "keep the account, refuse it at login",
		Run:         cmdDeactivateTrainer,
	},
	{
		Name: "reactivate-trainer", Group: groupAccounts, Args: "<username>",
		Description: "let a deactivated trainer back in",
		Run:         cmdReactivateTrainer,
	},
	{
		Name: "delete-trainer", Group: groupAccounts, Args: "<username>",
		Description: "remove the account for good, after asking",
		Run:         cmdDeleteTrainer,
	},
	{
		Name: "list-trainers", Group: groupAccounts,
		Description: "show every trainer and who lost access when",
		Run:         cmdListTrainers,
	},
	{
		Name: "import-athletes", Group: groupData, Args: "<file.csv>",
		Description: "add athletes to the roster from a CSV file",
		Run:         cmdImportAthletes,
	},
	{
		Name: "seed-demo", Group: groupDevelopment,
		Description: "put the demo athletes on the roster",
		Run:         cmdSeedDemo,
	},
	{
		Name: "clear-demo", Group: groupDevelopment,
		Description: "take the demo athletes off again",
		Run:         cmdClearDemo,
	},
}

func lookupCommand(name string) (command, bool) {
	for _, c := range commands {
		if c.Name == name {
			return c, true
		}
	}
	return command{}, false
}

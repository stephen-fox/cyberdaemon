package cyberdaemon

type CommandError struct {
	reason    string
	command   Command
	isUnknown bool
}

func (o CommandError) Error() string {
	if o.isUnknown {
		return "Unknown command - '" + o.command.string() + "'"
	}

	return o.reason
}

func (o CommandError) IsUnknownCommand() bool {
	return o.isUnknown
}

package cyberdaemon

import (
	"strings"
)

const (
	daemonStatusPrefix = "daemon status - "

	Unknown      Status = "unknown"
	Running      Status = "running"
	Stopped      Status = "stopped"
	Starting     Status = "starting"
	Stopping     Status = "stopping"
	Resuming     Status = "resuming"
	Pausing      Status = "pausing"
	Paused       Status = "paused"
	NotInstalled Status = "not_installed"

	GetStatus Command = "status"
	Start     Command = "start"
	Stop      Command = "stop"
	Install   Command = "install"
	Uninstall Command = "uninstall"
)

type Status string

func (o Status) printableStatus() string {
	return daemonStatusPrefix + string(o)
}

func (o Status) string() string {
	return string(o)
}

type Command string

func (o Command) string() string {
	return string(o)
}

type Daemon interface {
	Status() (Status, error)
	ExecuteCommand(Command) (string, error)
	BlockAndRun(ApplicationLogic) error
}

type ApplicationLogic interface {
	Start() error
	Stop() error
}

type Config struct {
	Name        string
	Description string
	Username    string
}

func CommandsString() string {
	return "'" + strings.Join(Commands(), "', '") + "'"
}

func Commands() []string {
	return []string{
		GetStatus.string(),
		Start.string(),
		Stop.string(),
		Install.string(),
		Uninstall.string(),
	}
}

func executedCommandMessage(command Command) string {
	return "Executed '" + command.string() + "' daemon control command"
}

package cyberdaemon

import (
	"strings"
)

const (
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

func (o Status) String() string {
	return string(o)
}

type Command string

func (o Command) string() string {
	return string(o)
}

type Daemon interface {
	Status() (Status, error)
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Execute(Command) (output string, err error)
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

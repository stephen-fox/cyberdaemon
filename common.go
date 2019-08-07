package cyberdaemon

import (
	"fmt"
	"strings"
)

const (
	Unknown      Status = "unknown"
	Running      Status = "running"
	Stopped      Status = "stopped"
	StoppedDead  Status = "stopped_dead"
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

// Status represents the status of a daemon.
type Status string

func (o Status) String() string {
	return string(o)
}

// Command represents a command that can be issued to a daemon, or that can
// control a daemon.
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
	RunUntilExit(ApplicationLogic) error
}

type ApplicationLogic interface {
	Start() error
	Stop() error
}

type Config struct {
	DaemonId    string
	Description string
	Username    string
	LogConfig   LogConfig
}

type LogConfig struct {
	UseNativeLogger bool
	NativeLogFlags  int
}

// SupportedCommandsString returns a printable string that represents a list of
// supported daemon control commands.
func SupportedCommandsString() string {
	return fmt.Sprintf("'%s'", strings.Join(SupportedCommands(), "', '"))
}

// SupportedCommands returns a slice of supported daemon control commands.
func SupportedCommands() []string {
	return []string{
		GetStatus.string(),
		Start.string(),
		Stop.string(),
		Install.string(),
		Uninstall.string(),
	}
}

// Execute executes a daemon control command for the provided daemon.
func Execute(command Command, daemon Daemon) (output string, err error) {
	switch command {
	case GetStatus:
		status, err := daemon.Status()
		if err != nil {
			return "", fmt.Errorf("failed to get daemon status - %s", err.Error())
		}

		return status.String(), nil
	case Start:
		err := daemon.Start()
		if err != nil {
			return "", fmt.Errorf("failed to start daemon - %s", err.Error())
		}

		return "", nil
	case Stop:
		err := daemon.Stop()
		if err != nil {
			return "", fmt.Errorf("failed to stop daemon - %s", err.Error())
		}

		return "", nil
	case Install:
		err := daemon.Install()
		if err != nil {
			return "", fmt.Errorf("failed to install daemon - %s", err.Error())
		}

		return "", nil
	case Uninstall:
		err := daemon.Uninstall()
		if err != nil {
			return "", fmt.Errorf("failed to uninstall daemon - %s", err.Error())
		}

		return "", nil
	}

	return "", fmt.Errorf("unknown daemon command '%s'", command.string())
}

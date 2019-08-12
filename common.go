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

	// StartImmediately means that the daemon will start immediately
	// after it is installed, and will be started whenever the
	// operating system loads the daemon (i.e., the behavior of the
	// 'StartOnLoad' option).
	//
	// See StartOnLoad for a detailed explanation of this behavior.
	StartImmediately StartType = "start_immediately"

	// StartOnLoad means that the daemon will not start after its
	// installation completes (with the exception of macOS). It will,
	// however, start each subsequent time the operating system loads
	// the daemon. Each operating system "loads" the daemon at slightly
	// different points depending on who owns the daemon (i.e., the
	// system itself, or a normal user). This is explained in further
	// detail below.
	//
	// On Linux - the answer is a bit complicated. On System V (init.d),
	// the daemon will start when the operating system boots. This will
	// happen regardless of the daemon being run by root, or by a normal
	// user. On systemd machines, a system-owned daemon will start when
	// the operating system boots. However, a user-owned daemon will only
	// start when that user logs in.
	//
	// On macOS, this means the daemon will start when either the system
	// boots (a system daemon), or when a user logs in (a user agent).
	// Due to the way launchd works, the daemon will be started after
	// installation finishes if this option is specified.
	//
	// On Windows, the daemon will start when the operating system
	// boots. If the daemon is configured to run as a normal user,
	// it will only start when the user logs in.
	StartOnLoad StartType = "start_on_load"

	// ManualStart means that the daemon must be started manually
	// after its installation completes. In addition, the operating
	// system will not start the daemon when it loads it.
	ManualStart StartType = "manual"
)

// Status represents the status of a daemon.
type Status string

func (o Status) String() string {
	return string(o)
}

// Command represents a command that can be issued to a daemon Controller.
type Command string

func (o Command) string() string {
	return string(o)
}

// StartType represents how the daemon will start once its installation
// is finished.
type StartType string

func (o StartType) string() string {
	return string(o)
}

// Daemonizer provides methods for daemonizing your application code.
//
// Daemonizer does not control the state of a daemon (e.g., installation
// or starting the daemon). See the Controller interface for daemon
// control functionality.
type Daemonizer interface {
	// RunUntilExit runs the provided Application until the daemon
	// is instructed to quit.
	RunUntilExit(Application) error
}

type Controller interface {
	Status() (Status, error)
	Install() error
	Uninstall() error
	Start() error
	Stop() error
}

// TODO: Additional daemon configuration:
//  - Manually setting daemon executable file path
//  - Support OS specific options
//  - Optionally remove configuration file if install fails
//  - Make the 'RunAs' field functional
//  - Optionally require that the daemon be stopped after uninstall?
type ControllerConfig struct {
	DaemonID    string
	Description string
	RunAs       string
	StartType   StartType
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

// Execute executes a control command using the provided daemon controller.
func Execute(command Command, controller Controller) (output string, err error) {
	switch command {
	case GetStatus:
		status, err := controller.Status()
		if err != nil {
			return "", fmt.Errorf("failed to get daemon status - %s", err.Error())
		}

		return status.String(), nil
	case Start:
		err := controller.Start()
		if err != nil {
			return "", fmt.Errorf("failed to start daemon - %s", err.Error())
		}

		return "", nil
	case Stop:
		err := controller.Stop()
		if err != nil {
			return "", fmt.Errorf("failed to stop daemon - %s", err.Error())
		}

		return "", nil
	case Install:
		err := controller.Install()
		if err != nil {
			return "", fmt.Errorf("failed to install daemon - %s", err.Error())
		}

		return "", nil
	case Uninstall:
		err := controller.Uninstall()
		if err != nil {
			return "", fmt.Errorf("failed to uninstall daemon - %s", err.Error())
		}

		return "", nil
	}

	return "", fmt.Errorf("unknown daemon command '%s'", command.string())
}

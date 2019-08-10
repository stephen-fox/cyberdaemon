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

	// ManualStart means that the daemon must be started manually
	// after its installation completes..
	ManualStart StartType = "manual"

	// StartOnLoad means that the daemon will start when it is loaded
	// by the operating system. The daemon will not start after the
	// installation completes (with the exception of macOS, which is
	// explained below). This behavior varies from one operating system
	// to another.
	//
	// On macOS, this means the daemon will start when either the system
	// boots (a system daemon), or when a user logs in (a user agent).
	// Due to the way launchd works, the daemon will be started after
	// installation finishes if this option is specified.
	//
	// On Linux - the answer is a bit more complicated. On System V
	// (init.d), the daemon will start when the operating system boots.
	// This will happen regardless of the daemon being run by root,
	// or by a normal user. On systemd machines, the daemon will start
	// when the operating system boots. However, a user-owned daemon
	// will only start when that user logs in.
	//
	// On Windows, the daemon will start when the operating system
	// boots. If the daemon is configured to run as a normal user,
	// it will start when the user logs in.
	StartOnLoad StartType = "start_on_load"

	// StartImmediately means that the daemon will start immediately
	// after it is installed, and will be subsequently started when the
	// operating system loads the daemon.
	//
	// See StartOnLoad for a detailed explanation of this behavior.
	StartImmediately StartType = "start_immediately"
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

// StartType represents how the daemon will start once its installation
// is finished.
type StartType string

func (o StartType) string() string {
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

// TODO: Additional daemon configuration:
//  - Manually setting daemon executable file path
//  - Support OS specific options
type Config struct {
	DaemonId    string
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

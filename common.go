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
// See the Controller interface for controlling the state of a daemon.
type Daemonizer interface {
	// RunUntilExit runs the provided Application until the daemon
	// is instructed to quit.
	RunUntilExit(Application) error
}

// Controller is an interface for controlling the state of a daemon.
//
// Be advised: Changing the state of a daemon requires super user privileges
// in the following scenarios:
//  - System daemons on all operating systems
//  - User-run Windows daemons
//  - User-run System V daemons
//
// See the Daemonizer interface for turning your application into
// a daemon.
type Controller interface {
	// Status returns the current status of the daemon.
	Status() (Status, error)

	// Install installs the daemon.
	Install() error

	// Uninstall stops and uninstalls the daemon.
	Uninstall() error

	// Start starts the daemon.
	Start() error

	// Stop stops the daemon.
	Stop() error
}

// ControllerConfig configures a daemon Controller.
//
// TODO: Additional daemon configuration:
//  - Manually setting daemon executable file path
//  - Support OS specific options
//  - Optionally remove configuration file if install fails
//  - Make the 'RunAs' field functional
//  - Optionally require that the daemon be stopped after uninstall?
type ControllerConfig struct {
	// DaemonID is the string used to identify a daemon (for example,
	// "MyApp"). The string must follow these rules:
	//  - Contain no spaces or special characters
	//  - On macOS, must be in reverse DNS format (e.g.,
	//    com.github.thedude.myapp)
	DaemonID string

	// Description is a short blurb describing your application.
	Description string

	// RunAs is the user to run the daemon as.
	//
	// If left unset, the daemon will run as the following:
	//  - root on unix systems
	//  - Administrator on Windows systems
	RunAs string

	// StartType specifies the daemon's start up behavior.
	//
	// If left unset, the daemon must be started manually.
	StartType StartType

	// LogConfig configures the logging settings for the daemon.
	LogConfig LogConfig
}

// LogConfig configures the logging settings for the daemon.
type LogConfig struct {
	// UseNativeLogger specifies whether the operating system's native
	// logging tool should be used. When set to 'true', implementers
	// should use the standard library's 'log' package to facilitate
	// logging. This guarantees that your log messages will reach the
	// native logging utility.
	//
	// For Linux systems, the native logger depends on whether systemd
	// or System V is used. Systemd saves stderr output from the daemon.
	// These logs are accessed by running:
	//  journalctl -u myapp
	// You can add '-f' to the above command to display log messages
	// as they are created.
	// System V (init.d), however, does not provide a similar logging
	// tool. If the daemon was installed using a Controller, the stderr
	// output of the daemon will be redirected to a log file. This log
	// file can be found at:
	//  /var/log/myapp/myapp.log
	//
	// macOS, like System V, does not provide a logging tool. The stderr
	// output of your daemon will be redirected to:
	//  User daemon: ~/Library/Logs/com.github.myapp/com.github.myapp.log
	//  System daemon: /Library/Logs/com.github.myapp/com.github.myapp.log
	//
	// Windows provides the Event Log utility for saving log messages.
	// Log messages can be viewed using either the 'Event Viewer' GUI
	// application, or by running:
	//  TODO: Event viewer CLI command
	UseNativeLogger bool

	// NativeLogFlags specifies which log flags to use when UserNativeLogger
	// is set to 'true'. The value must be greater than zero to take effect.
	// See the standard library's 'log' package for more information about
	// log flags.
	NativeLogFlags int
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

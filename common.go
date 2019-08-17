package cyberdaemon

// Daemonizer provides methods for daemonizing your application code.
//
// Gotchas
//
// There are several "gotchas" that implementers should be aware of when using
// the Daemonizer.
//
// System V (init.d) on Linux:
// A file known as a "PID file" is used to store the process ID of the running
// daemon's process. Both the init.d script, and daemon must know where this
// file is stored - the script to read it, and the daemon to update it. By
// default, the System V Daemonizer will attempt to find the PID file by
// looking in the init.d script for a Bash variable named:
// 	'PID_FILE_PATH'
// If it cannot locate such a variable, it will use:
// 	"/var/run/<init.d-script-name>.pid"
//
// Windows:
// On Windows, developers must implement a method named 'WindowsDaemonID'
// in their Application implementations. The Windows build of this interface
// differs from the unix variant by this one method. This is needed because
// the Windows service manager needs to know the service name when starting
// the daemon. This was implemented in the Application interface to make it
// a compile-time error if developers try to build for Windows in addition
// to other operating systems.
type Daemonizer interface {
	// RunUntilExit runs the provided Application until the daemon
	// is instructed to quit.
	RunUntilExit(Application) error
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
	// 	journalctl -u myapp
	// You can add '-f' to the above command to display log messages
	// as they are created.
	// System V (init.d), however, does not provide a similar logging
	// tool. If the daemon was installed using a Controller, the stderr
	// output of the daemon will be redirected to a log file. This log
	// file can be found at:
	// 	/var/log/myapp/myapp.log
	// If a System V daemon was not installed using a controller, it will
	// attempt to output logs to stderr.
	//
	// macOS, like System V, does not provide a logging tool. If the daemon
	// was installed using a Controller, its stderr will be redirected to:
	// 	/Library/Logs/com.github.myapp/com.github.myapp.log
	// ... and user daemons will be saved to:
	// 	~/Library/Logs/com.github.myapp/com.github.myapp.log
	// If a macOS daemon was not installed using a controller, it will
	// attempt to output logs to stderr.
	//
	// Windows provides the Event Log utility for saving log messages.
	// Log messages can be viewed using either the 'Event Viewer' GUI
	// application, or by running:
	// 	TODO: Event viewer CLI command
	UseNativeLogger bool

	// NativeLogFlags specifies which log flags to use when UserNativeLogger
	// is set to 'true'. The value must be greater than zero to take effect.
	// See the standard library's 'log' package for more information about
	// log flags.
	NativeLogFlags int
}

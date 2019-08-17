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
//
// Implementation details
//
// System V (init.d):
// System V is one of the more complicated daemon implementations in this
// library. This is due to Go's inability to fork, and the tight-knit nature of
// the daemon management script and its corresponding daemon application.
//
// System V works by using a script (typically written in Bourne Shell) to
// manage the daemon. This script is usually stored in '/etc/init.d'. System V
// expects the init.d script to exit after starting the daemon process. This is
// typically implemented using the 'fork' system call, which spawns a new
// process with only one thread. Go's runtime requires multiple threads to run
// (many Go programs rely on more than one thread anyways). An alternate
// solution is to do what is called "fork exec". This means the Go program runs
// a new instance of itself, and the original instance exits. Implementing this
// is tricky because the library cannot rely on settings files, command line
// arguments, or environment variables to determine if init.d started it. Doing
// so would cross an implementation line that would require implementers to
// bake the special business logic into their own independent init.d scripts.
//
// This library's System V daemon implementation checks the parent PID's
// "/proc/<pid>/cmdline" to determine:
// 	- Is init.d the parent process?
// 	- If so, where is the init.d script stored?
//
// If started by init.d, the daemon will attempt to parse the PID file path
// from the init.d script. A PID file is used to store the PID of the daemon
// so that the management script can easily determine the status of the daemon.
// Both the init.d script and the daemon need to know where this file is
// located (the script so that it can read it, and the daemon so that it can
// write its PID to it). If the daemon cannot find the PID file path in the
// init.d script, it uses a sane default PID file path.
type Daemonizer interface {
	// RunUntilExit runs the provided Application until the daemon is
	// instructed to quit. This method blocks until the daemon exits.
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

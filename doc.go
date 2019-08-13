// Package cyberdaemon provides tooling for creating and managing a
// platform-agnostic daemon.
//
// Supported systems
//
// Linux:
// 	- systemd
// 	- System V (init.d)
// macOS:
// 	- launchd
// Windows:
// 	- Windows service
//
// Usage
//
// This library provides three primary interfaces for creating and managing
// a daemon:
// 	- Controller
// 	- Daemonizer
// 	- Application
//
// The Controller is used to control the state of a daemon. It is used to
// communicate with the operating system's daemon management software to
// query a daemon's status, start or stop it, and install and uninstall it.
// A Controller is configured using the ControllerConfig struct.
//
// Daemonizer is used to turn your application into a daemon. Implementations
// of this interface use operating system specific calls and logic to properly
// run your code as a daemon. This is facilitated by the Application interface.
// Usage of a Controller is not required when using a Daemonizer. You may
// implement your own daemon management tooling while leveraging the Daemonizer
// to run your application.
//
// The Application interface is used by the Daemonizer to run your application
// code as a daemon. Implement this interface in your application and use the
// Daemonizer to run your program.
//
// Daemonizer implementation details
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
// from the init.d script by finding a variable named 'PID_FILE_PATH'. A PID
// file is used to store the PID of the daemon so that the management script
// can easily determine the status of the daemon. Both the init.d script and
// the daemon need to know where this file is located (the script so that it
// can read it, and the daemon so that it can write its PID to it). If the
// daemon cannot find the PID file path in the init.d script, it uses
// "/var/run/<daemon-id>/<daemon-id>.pid" as the path. It then opens the file
// and starts a new instance of itself, which it passes the open PID file
// descriptor to the new instance.
package cyberdaemon

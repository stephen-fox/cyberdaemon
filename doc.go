// Package cyberdaemon provides tooling for creating and managing a
// platform-agnostic daemon.
//
// Supported systems
//
// 	- Linux
// 		- systemd
// 		- System V (init.d) (if systemd is unavailable)
// 	- macOS
// 		- launchd
// 	- Windows
// 		- Windows service
//
// Usage
//
// The top-level package provides the following interfaces:
// 	- Daemonizer
// 	- Application
//
// Daemonizer is used to turn your application into a daemon. Implementations
// of this interface use operating system specific calls and logic to properly
// run your code as a daemon. This is facilitated by the Application interface.
// Usage of the 'control' subpackage is not required when using a Daemonizer.
// Developers may implement their own daemon management tooling while also
// leveraging the Daemonizer. Please review the 'Gotchas' documentation in the
// Daemonizer interface if you choose to use your own management tooling.
//
// The Application interface is used by the Daemonizer to run your application
// code as a daemon. Implement this interface in your application and use the
// Daemonizer to run your program.
//
// The 'cyberdaemon/control' subpackage provides tools for managing a daemon's
// state. Please review its package documentation for more information.
package cyberdaemon

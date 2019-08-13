package cyberdaemon

// Application provides an interface for a daemon to invoke your application
// code. The reasoning behind this design is that the daemon must first setup
// and configure daemon-specific settings (like logging) before your
// application can execute.
type Application interface {
	// Start is called when the daemon is ready to start your application.
	// Implementations of this method must start your application using
	// a goroutine (or some other asynchronous mechanism) and return as
	// quickly as possible. A non-nil error should be returned if the
	// application cannot be started.
	Start() error

	// Stop is called when the daemon is stopped by the operating system.
	// Implementations of this method must return as quickly as possible.
	// A non-nil error should be returned if a critical error occurs when
	// your application stops.
	Stop() error

	// WindowsDaemonID is used to identify the daemon. Windows requires
	// this string so it can start the corresponding Windows Service.
	// This string is the Windows Service's name.
	WindowsDaemonID() string
}

// Package control provides functionality for managing a daemon.
//
// The control subpackage provides the following interface:
// 	- Controller
//
// The Controller is used to control the state of a daemon. Implementations
// communicate with the operating system's daemon management software to
// query a daemon's status, start or stop it, and install and uninstall it.
// A Controller is configured using the ControllerConfig struct. This struct
// provides the necessary information about a daemon (such as its ID).
// It also provides customization options, such as the start up type.
package control

package cyberdaemon

// TODO: Documentation.
type Application interface {
	Start() error
	Stop() error
	WindowsDaemonID() string
}

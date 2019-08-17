package cyberdaemon

const (
	// PasswordOption is used to specify the password for a user when
	// installing a daemon that will run as that user. The following
	// example ControllerConfig demonstrates this option by reading
	// the user's password from an environment variable:
	//
	//	config := cyberdaemon.ControllerConfig{
	//		DaemonID:              "test",
	//		Description:           "I need my guys. They're the best.",
	//		RunAs:                 ".\\stephen",
	//		SystemSpecificOptions: map[cyberdaemon.SystemSpecificOption]interface{}{
	//			cyberdaemon.PasswordOption: cyberdaemon.GetPassword(func() (string, error) {
	//				p, ok := os.LookupEnv("WHYARETHEYTHEBEST")
	//				if !ok {
	//					return "", fmt.Errorf("password environment variable was not set")
	//				}
	//				return p, nil
	//			}),
	//		},
	//	}
	PasswordOption SystemSpecificOption = "password"
)

// GetPassword represents a function that will return a user's password when
// installing a daemon that will run as that user. See the documentation for
// PasswordOption for more information.
type GetPassword func() (string, error)

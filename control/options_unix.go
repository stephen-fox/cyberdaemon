// +build !windows

package control

const (
	// RunOnlyWhenLoggedIn specifies that the daemon should run only
	// when the daemon's owner is logged in. The 'RunAs' field in the
	// ControllerConfig must be set to the username that will own the
	// daemon. This options does not take effect if the 'RunAs' field
	// is not set. This option is not supported on System V.
	//
	// The following ControllerConfig example demonstrates how to
	// specify this option:
	//
	//	current, err := user.Current()
	//	if err != nil {
	//		return err
	//	}
	//
	//	config := control.ControllerConfig{
	//		DaemonID:              "test",
	//		Description:           "I need my guys. They're the best.",
	//		RunAs:                 current.Username,
	//		SystemSpecificOptions: map[control.SystemSpecificOption]interface{}{
	//			control.RunOnlyWhenLoggedIn: "",
	//		},
	//	}
	RunOnlyWhenLoggedIn SystemSpecificOption = "run_only_when_logged_in"
)

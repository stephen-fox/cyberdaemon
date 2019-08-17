package control

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"

	"github.com/coreos/go-systemd/unit"
	"github.com/stephen-fox/cyberdaemon/internal/osutil"
)

const (
	systemctlExeName    = "systemctl"
	userArgument        = "--user"
	daemonReloadCommand = "daemon-reload"
)

var (
	systemctlExeDirPaths = []string{"/bin"}
)

type systemdController struct {
	systemctlPath string
	daemonID      string
	unitFilePath  string
	unitContents  []byte
	addUserArg    bool
	startType     StartType
}

func (o *systemdController) Status() (Status, error) {
	initInfo, statErr := os.Stat(o.unitFilePath)
	if statErr != nil || initInfo.IsDir() {
		return NotInstalled, nil
	}

	var args []string
	if o.addUserArg {
		args = append(args, userArgument)
	}
	args = append(args,"status", o.daemonID)

	_, exitCode, statusErr := osutil.RunDaemonCli(o.systemctlPath, args...)
	if statusErr != nil {
		switch exitCode {
		case 3:
			return Stopped, nil
		case 1:
			return StoppedDead, nil
		}
	}

	if exitCode == 0 {
		return Running, nil
	}

	return Unknown, nil
}

func (o *systemdController) Install() error {
	err := ioutil.WriteFile(o.unitFilePath, o.unitContents, 0644)
	if err != nil {
		return fmt.Errorf("failed to write systemd unit file - %s", err.Error())
	}

	var args []string
	if o.addUserArg {
		args = append(args, userArgument)
	}

	switch o.startType {
	case StartImmediately:
		_, _, err = osutil.RunDaemonCli(o.systemctlPath, append(args, daemonReloadCommand)...)
		if err != nil {
			return err
		}
		err := o.Start()
		if err != nil {
			return err
		}
		fallthrough
	case StartOnLoad:
		_, _, err := osutil.RunDaemonCli(o.systemctlPath, append(args, "enable", o.daemonID)...)
		if err != nil {
			return err
		}
	case ManualStart:
	}

	return nil
}

func (o *systemdController) Uninstall() error {
	// Try to stop the daemon. Ignore any errors because it might be
	// stopped already, or the stop failed (which there is nothing
	// we can do.
	o.Stop()

	err := os.Remove(o.unitFilePath)
	if err != nil {
		return err
	}

	var args []string
	if o.addUserArg {
		args = append(args, userArgument)
	}
	args = append(args, daemonReloadCommand)

	_, _, err = osutil.RunDaemonCli(o.systemctlPath, args...)
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdController) Start() error {
	var args []string
	if o.addUserArg {
		args = append(args, userArgument)
	}
	args = append(args, "start", o.daemonID)

	_, _, err := osutil.RunDaemonCli(o.systemctlPath, args...)
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdController) Stop() error {
	var args []string
	if o.addUserArg {
		args = append(args, userArgument)
	}
	args = append(args, "stop", o.daemonID)

	_, _, err := osutil.RunDaemonCli(o.systemctlPath, args...)
	if err != nil {
		return err
	}

	return nil
}

func newSystemdController(exePath string, config ControllerConfig, systemctlPath string) (*systemdController, error) {
	command := exePath
	if len(config.Arguments) > 0 {
		command = fmt.Sprintf("%s %s", exePath, config.argumentsAsString())
	}

	unitOptions := []*unit.UnitOption{
		{
			Section: "Unit",
			Name:    "Description",
			Value:   config.Description,
		},
		{
			Section: "Service",
			Name:    "Type",
			Value:   "simple",
		},
		{
			Section: "Service",
			Name:    "ExecStart",
			Value:   command,
		},
		{
			Section: "Service",
			Name:    "Restart",
			Value:   "on-failure",
		},
		{
			Section: "Install",
			Name:    "WantedBy",
			Value:   "multi-user.target",
		},
	}

	addUserToUnit, unitFilePath, specifyUserArg, err := runSettings(config)
	if err != nil {
		return nil, err
	}

	if addUserToUnit {
		unitOptions = append(unitOptions, unit.NewUnitOption("Service", "User", config.RunAs))
	}

	unitContents, err := ioutil.ReadAll(unit.Serialize(unitOptions))
	if err != nil {
		return nil, fmt.Errorf("failed to read from unit reader - %s", err.Error())
	}

	return &systemdController{
		systemctlPath: systemctlPath,
		daemonID:      config.DaemonID,
		unitFilePath:  unitFilePath,
		unitContents:  unitContents,
		addUserArg:    specifyUserArg,
		startType:     config.StartType,
	}, nil
}

// runSettings returns whether the user should be specified in the unit config
// file, the unit file path, and whether '--user' needs to be specified when
// running the 'systemctl' command.
func runSettings(config ControllerConfig) (bool, string, bool, error) {
	defaultUnitPath := fmt.Sprintf("/etc/systemd/system/%s.service", config.DaemonID)
	if len(config.RunAs) == 0 {
		return false, defaultUnitPath, false, nil
	}

	current, err := user.Current()
	if err != nil {
		return false, "", false, fmt.Errorf("failed to get current user - %s", err.Error())
	}

	_, onlyRunWhenLoggedIn := config.SystemSpecificOptions[RunOnlyWhenLoggedIn]
	if onlyRunWhenLoggedIn {
		if config.RunAs == current.Username {
			return false, fmt.Sprintf("%s/.config/systemd/user/%s.service", current.HomeDir, config.DaemonID),
				true, nil
		}
		return false, "", false,
			fmt.Errorf("the '%s' option cannot be used when the curret user is not the RunAs user",
				RunOnlyWhenLoggedIn)
	}

	return true, defaultUnitPath, false, nil
}

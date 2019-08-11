package cyberdaemon

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/unit"
)

const (
	systemctlExeName = "systemctl"
)

var (
	systemctlExeDirPaths = []string{"/bin"}
)

// TODO: Support running as a different user ('--user').
type systemdController struct {
	systemctlPath string
	daemonId      string
	unitFilePath  string
	unitContents  []byte
	startType     StartType
}

func (o *systemdController) Status() (Status, error) {
	initInfo, statErr := os.Stat(o.unitFilePath)
	if statErr != nil || initInfo.IsDir() {
		return NotInstalled, nil
	}

	_, exitCode, statusErr := runDaemonCli(o.systemctlPath,"status", o.daemonId)
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

	switch o.startType {
	case StartImmediately:
		// TODO: This only works for system-level units. This needs to use
		//  'systemctl --user daemon-reload' when dealing with userland.
		_, _, err = runDaemonCli(o.systemctlPath, "daemon-reload")
		if err != nil {
			return err
		}
		err := o.Start()
		if err != nil {
			return err
		}
		fallthrough
	case StartOnLoad:
		_, _, err := runDaemonCli(o.systemctlPath, "enable", o.daemonId)
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

	// TODO: This only works for system-level units. This needs to use
	//  'systemctl --user daemon-reload' when dealing with userland.
	_, _, err = runDaemonCli(o.systemctlPath, "daemon-reload")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdController) Start() error {
	_, _, err := runDaemonCli(o.systemctlPath, "start", o.daemonId)
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdController) Stop() error {
	_, _, err := runDaemonCli(o.systemctlPath, "stop", o.daemonId)
	if err != nil {
		return err
	}

	return nil
}

type systemdDaemonizer struct {
	logConfig LogConfig
}

func (o *systemdDaemonizer) RunUntilExit(application Application) error {
	// Only do native log things when running non-interactively.
	// The 'PS1' environment variable will be empty / not set when
	// this is run non-interactively.
	if o.logConfig.UseNativeLogger && len(os.Getenv("PS1")) == 0 {
		log.SetOutput(os.Stderr)
		// systemd logs automatically append a timestamp. We can
		// disable the go logger's timestamp by setting log flags
		// to 0.
		log.SetFlags(0)

		if o.logConfig.NativeLogFlags > 0 {
			originalLogFlags := log.Flags()
			log.SetFlags(o.logConfig.NativeLogFlags)
			defer log.SetFlags(originalLogFlags)
		}
	}

	err := application.Start()
	if err != nil {
		return err
	}

	interruptsAndTerms := make(chan os.Signal)
	signal.Notify(interruptsAndTerms, os.Interrupt, syscall.SIGTERM)
	<-interruptsAndTerms
	signal.Stop(interruptsAndTerms)

	return application.Stop()
}

func newSystemdController(exePath string, config Config, systemctlPath string) (*systemdController, error) {
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
			Value:   exePath,
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

	unitContents, err := ioutil.ReadAll(unit.Serialize(unitOptions))
	if err != nil {
		return nil, fmt.Errorf("failed to read from unit reader - %s", err.Error())
	}

	return &systemdController{
		systemctlPath: systemctlPath,
		daemonId:      config.DaemonId,
		unitFilePath:  fmt.Sprintf("/etc/systemd/system/%s.service", config.DaemonId),
		unitContents:  unitContents,
		startType:     config.StartType,
	}, nil
}

func newSystemdDaemonizer(logConfig LogConfig) Daemonizer {
	return &systemdDaemonizer{
		logConfig: logConfig,
	}
}

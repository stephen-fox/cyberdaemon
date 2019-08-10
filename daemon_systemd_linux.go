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
	defaultSystemdExePath = "/bin/systemctl"
)

// TODO: Support systemctl enable / disable.
type systemdDaemon struct {
	systemctlPath string
	daemonId      string
	unitFilePath  string
	unitContents  []byte
	logConfig     LogConfig
}

func (o *systemdDaemon) Status() (Status, error) {
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

func (o *systemdDaemon) Install() error {
	err := ioutil.WriteFile(o.unitFilePath, o.unitContents, 0644)
	if err != nil {
		return fmt.Errorf("failed to write systemd unit file - %s", err.Error())
	}

	_, _, err = runDaemonCli(o.systemctlPath, "enable", o.daemonId)
	if err != nil {
		return err
	}

	// TODO: If user specifies to start after installing.
	err = o.Start()
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdDaemon) Uninstall() error {
	// TODO: Should we do this before uninstalling other daemons?
	o.Stop()

	return os.Remove(o.unitFilePath)
}

func (o *systemdDaemon) Start() error {
	_, _, err := runDaemonCli(o.systemctlPath, "start", o.daemonId)
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdDaemon) Stop() error {
	_, _, err := runDaemonCli(o.systemctlPath, "stop", o.daemonId)
	if err != nil {
		return err
	}

	return nil
}

func (o *systemdDaemon) RunUntilExit(logic ApplicationLogic) error {
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

	err := logic.Start()
	if err != nil {
		return err
	}

	interruptsAndTerms := make(chan os.Signal)
	signal.Notify(interruptsAndTerms, os.Interrupt, syscall.SIGTERM)
	<-interruptsAndTerms
	signal.Stop(interruptsAndTerms)

	return logic.Stop()
}

func newSystemdDaemon(exePath string, config Config, systemctlPath string) (*systemdDaemon, error) {
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

	return &systemdDaemon{
		systemctlPath: systemctlPath,
		daemonId:      config.DaemonId,
		logConfig:     config.LogConfig,
		unitFilePath:  fmt.Sprintf("/etc/systemd/system/%s.service", config.DaemonId),
		unitContents:  unitContents,
	}, nil
}
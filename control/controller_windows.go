package control

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"strconv"
	"time"
)

const (
	notInstalledErr = "The specified service does not exist as an installed service."
)

type windowsController struct {
	config       ControllerConfig
	winStartType uint32
}

func (o *windowsController) Status() (Status, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
	if err != nil {
		if err.Error() == notInstalledErr {
			return NotInstalled, nil
		}

		return "", err
	}
	defer s.Close()

	winStatus, err := s.Query()
	if err != nil {
		return "", err
	}

	switch winStatus.State {
	case svc.StopPending:
		return Stopping, nil
	case svc.Stopped:
		return Stopped, nil
	case svc.StartPending:
		return Starting, nil
	case svc.Running:
		return Running, nil
	case svc.ContinuePending:
		return Resuming, nil
	case svc.PausePending:
		return Pausing, nil
	case svc.Paused:
		return Paused, nil
	}

	return Unknown, nil
}

func (o *windowsController) Install() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	var password string
	if len(o.config.RunAs) > 0 {
		v, ok := o.config.SystemSpecificOptions[PasswordOption]
		if !ok {
			return fmt.Errorf("the '%s' operating system specific option must be specified to run a windows service as a normal user", PasswordOption)
		}

		getPassFn, ok := v.(GetPassword)
		if !ok {
			return fmt.Errorf("the '%s' option must be a GetPassword function (type assertion failure)", PasswordOption)
		}

		password, err = getPassFn()
		if err != nil {
			return fmt.Errorf("failed to get password when installing daemon - %s", err.Error())
		}
	}

	c := mgr.Config{
		DisplayName:      o.config.DaemonID,
		Description:      o.config.Description,
		StartType:        o.winStartType,
		ServiceStartName: o.config.RunAs,
		Password:         password,
	}

	s, err := m.CreateService(o.config.DaemonID, o.config.ExePath, c, o.config.Arguments...)
	if err != nil {
		return err
	}
	defer s.Close()

	if o.config.StartType == StartImmediately {
		err := s.Start()
		if err != nil {
			s.Delete()
			return err
		}
	}

	err = eventlog.InstallAsEventCreate(o.config.DaemonID, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return err
	}

	return nil
}

func (o *windowsController) Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
	if err != nil {
		return err
	}
	defer s.Close()

	// Attempt to stop the service before removing it.
	// Windows does not stop the service's process when
	// the service is deleted. Do not bother checking
	// the error because there is nothing to do if the
	// stop fails.
	stopAndWait(s)

	err = s.Delete()
	if err != nil {
		return err
	}

	err = eventlog.Remove(o.config.DaemonID)
	if err != nil {
		return err
	}

	return nil
}

func (o *windowsController) Start() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
	if err != nil {
		return err
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return err
	}

	return nil
}

func (o *windowsController) Stop() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
	if err != nil {
		return err
	}
	defer s.Close()

	err = stopAndWait(s)
	if err != nil {
		return err
	}

	return nil
}

func NewController(controllerConfig ControllerConfig) (Controller, error) {
	err := controllerConfig.Validate()
	if err != nil {
		return nil, err
	}

	var winStartType uint32
	switch controllerConfig.StartType {
	case StartImmediately, StartOnLoad:
		winStartType = mgr.StartAutomatic
	default:
		winStartType = mgr.StartManual
	}

	return &windowsController{
		config:       controllerConfig,
		winStartType: winStartType,
	}, nil
}

// stopAndWait based on stopAndWait by takama et al:
//
//  github.com/takama/daemon in daemon_windows.go
//  commit: 7b0f9893e24934bbedef065a1768c33779951e7d
func stopAndWait(s *mgr.Service) error {
	// First stop the service. Then wait for the service to
	// actually stop before starting it.
	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}

	timeDuration := time.Millisecond * 50

	timeout := defaultServiceStopTimeout()
	if timeout == 0 {
		timeout = time.Millisecond * 20000
	}

	onTimeout := time.After(timeout + (timeDuration * 2))
	tick := time.NewTicker(timeDuration)
	defer tick.Stop()

	for status.State != svc.Stopped {
		select {
		case <-tick.C:
			status, err = s.Query()
			if err != nil {
				return err
			}
		case <-onTimeout:
			return fmt.Errorf("service failed to stop after %s", timeout.String())
		}
	}

	return nil
}

// defaultServiceStopTimeout based on getStopTimeout by takama et al:
//
//  github.com/takama/daemon in daemon_windows.go
//  commit: 7b0f9893e24934bbedef065a1768c33779951e7d
func defaultServiceStopTimeout() time.Duration {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control`, registry.READ)
	if err != nil {
		return 0
	}

	sv, _, err := key.GetStringValue("WaitToKillServiceTimeout")
	if err != nil {
		return 0
	}

	v, err := strconv.Atoi(sv)
	if err != nil {
		return 0
	}

	return time.Millisecond * time.Duration(v)
}

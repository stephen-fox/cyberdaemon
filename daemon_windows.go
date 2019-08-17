package cyberdaemon

import (
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"log"
	"os"
	"os/signal"
	"sync"
)

type windowsDaemonizer struct {
	logConfig LogConfig
}

func (o *windowsDaemonizer) RunUntilExit(application Application) error {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return err
	}

	if isInteractive {
		err = application.Start()
		if err != nil {
			return err
		}

		interrupts := make(chan os.Signal)
		signal.Notify(interrupts, os.Interrupt)
		<-interrupts
		signal.Stop(interrupts)

		return application.Stop()
	}

	if o.logConfig.UseNativeLogger {
		events, err := eventlog.Open(application.WindowsDaemonID())
		if err != nil {
			return err
		}
		originalLogFlags := log.Flags()
		if o.logConfig.NativeLogFlags > 0 {
			log.SetFlags(o.logConfig.NativeLogFlags)
		} else {
			// Timestamps are provided by Windows event log by
			// default. Set log flags to 0, thus disabling the
			// go logger's timestamps.
			log.SetFlags(0)
		}
		log.SetOutput(&eventLogWriter{
			fn: events.Info,
		})
		defer log.SetFlags(originalLogFlags)
		defer log.SetOutput(os.Stderr)
		defer events.Close()
	}

	wrapper := serviceWrapper{
		name:     application.WindowsDaemonID(),
		app:      application,
		errMutex: &sync.Mutex{},
	}

	err = wrapper.runAndBlock()
	if err != nil {
		return err
	}

	return nil
}

type eventLogWriter struct {
	fn func(eventId uint32, message string) error
}

func (o eventLogWriter) Write(p []byte) (n int, err error) {
	err = o.fn(0, string(p))
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

type serviceWrapper struct {
	name     string
	app      Application
	errMutex *sync.Mutex
	lastErr  error
}

// runAndBlock based on windowsDaemon.Run() method by kardianos et al:
//
//  github.com/kardianos/service in service_windows.go
//  commit: b1866cf76903d81b491fb668ba14f4b1322b2ca7
func (o *serviceWrapper) runAndBlock() error {
	o.setStartStopError(nil)

	// Return error messages from start and stop routines
	// that get executed in the Execute method.
	// Guarded with a mutex as it may run a different thread
	// (callback from windows).
	runErr := svc.Run(o.name, o)

	startStopErr := o.startStopErr()
	if startStopErr != nil {
		return startStopErr
	}

	if runErr != nil {
		return runErr
	}

	return nil
}

// Execute based on windowsDaemon.Execute() method by kardianos et al:
//
//  github.com/kardianos/service in service_windows.go
//  commit: b1866cf76903d81b491fb668ba14f4b1322b2ca7
func (o *serviceWrapper) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop|svc.AcceptShutdown

	changes <- svc.Status{
		State: svc.StartPending,
	}

	if err := o.app.Start(); err != nil {
		o.setStartStopError(err)
		return true, 1
	}

	changes <- svc.Status{
		State:   svc.Running,
		Accepts: cmdsAccepted,
	}

loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{
				State: svc.StopPending,
			}
			if err := o.app.Stop(); err != nil {
				o.setStartStopError(err)
				return true, 2
			}
			break loop
		default:
			continue loop
		}
	}

	return false, 0
}

func (o *serviceWrapper) setStartStopError(err error) {
	o.errMutex.Lock()
	o.lastErr = err
	o.errMutex.Unlock()
}

func (o *serviceWrapper) startStopErr() error {
	o.errMutex.Lock()
	defer o.errMutex.Unlock()

	return o.lastErr
}

func NewDaemonizer(logConfig LogConfig) Daemonizer {
	return &windowsDaemonizer{
		logConfig: logConfig,
	}
}

package cyberdaemon

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

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

func newSystemdDaemonizer(logConfig LogConfig) Daemonizer {
	return &systemdDaemonizer{
		logConfig: logConfig,
	}
}

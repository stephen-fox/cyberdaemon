package main

// This is an example daemon. Run with '-h' for more information.

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/stephen-fox/cyberdaemon"
)

const (
	appName     = "cyberdaemon-filewriter-example"
	description = "An example daemon implemented using the cyberdaemon library."

	daemonCommandArg = "command"
	example = "example.exe -" + daemonCommandArg

	usage = appName + `

[ABOUT]
This is an example daemon implemented using the cyberdaemon library. It will
update a file every few seconds in the OS's temporary directory. This is
'/tmp' on *nix, or 'C:\Windows\Temp' (the value of the 'TEMP' environment
variable) on Windows.

Compile this application as an executable (make sure the executable's name ends
with .exe if you are on Windows). You can then install it as a daemon
by running:
	'` + example + ` install'

Once installed, the daemon can be stopped or started by running:
	'` + example + ` stop'
or:
	'` + example + ` start'

The daemon can be uninstalled by running:
	'` + example + ` uninstall'

[USAGE]`
)

func main() {
	command := flag.String(daemonCommandArg, "", "The daemon command to execute. This can be the following:\n" +
		cyberdaemon.SupportedCommandsString())
	help := flag.Bool("h", false, "Displays this help page")

	flag.Parse()

	if *help {
		fmt.Println(usage)
		flag.PrintDefaults()
		os.Exit(0)
	}

	daemonID := appName
	if runtime.GOOS == "darwin" {
		daemonID = fmt.Sprintf("com.github.stephen-fox.%s", appName)
	}
	logConfig := cyberdaemon.LogConfig{
		UseNativeLogger: true,
	}

	if len(*command) > 0 {
		controller, err := cyberdaemon.NewController(cyberdaemon.Config{
			DaemonID:    daemonID,
			Description: description,
			StartType:   cyberdaemon.StartImmediately,
			LogConfig:   logConfig,
		})
		if err != nil {
			log.Fatalln(err.Error())
		}

		output, err := cyberdaemon.Execute(cyberdaemon.Command(*command), controller)
		if err != nil {
			log.Fatalln(err.Error())
		}

		if len(output) > 0 {
			log.Println(output)
		}

		return
	}

	daemon := cyberdaemon.NewDaemonizer(logConfig)
	err := daemon.RunUntilExit(&application{
		daemonID: daemonID,
		stop:     make(chan chan struct{}),
	})
	if err != nil {
		log.Fatalln(err.Error())
	}
}

type application struct {
	daemonID string
	stop     chan chan struct{}
}

func (o *application) Start() error {
	workDirPath := "/tmp"
	if runtime.GOOS == "windows" {
		workDirPath = os.Getenv("TEMP")
	}
	workDirPath = path.Join(workDirPath, appName)

	err := os.MkdirAll(workDirPath, 0755)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path.Join(workDirPath, "filewriter-output.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	go updateFileLoop(f, o.stop)

	return nil
}

func (o *application) Stop() error {
	stopTimeout := 5 * time.Second
	onStopTimeout := time.NewTimer(stopTimeout)
	rejoin := make(chan struct{})

	log.Println("stopping...")

	select {
	case o.stop <- rejoin:
		onStopTimeout.Stop()
		rejoinTimeout := 2 * time.Second
		onRejoinTimeout := time.NewTimer(rejoinTimeout)

		select {
		case <-rejoin:
			log.Println("rejoined on stop")
			onRejoinTimeout.Stop()
		case <-onRejoinTimeout.C:
			return fmt.Errorf("application did not stop after %s", rejoinTimeout.String())
		}
	case <-onStopTimeout.C:
		return fmt.Errorf("application did not respond to stop after %s", stopTimeout.String())
	}

	return nil
}

func (o *application) WindowsDaemonID() string {
	return o.daemonID
}

func updateFileLoop(f *os.File, stop chan chan struct{}) {
	defer f.Close()
	defer os.Remove(f.Name())

	log.Printf("target file is '%s'", f.Name())

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := fmt.Fprintln(f, time.Now().String())
			if err != nil {
				log.Printf("failed to write to file - %s", err.Error())
			}
		case rejoin := <-stop:
			log.Printf("received stop signal")
			rejoin <- struct{}{}
			return
		}
	}
}

# cyberdaemon
A Go library that provides tooling for creating and managing a
platform-agnostic daemon.

## Use cases
My primary motivation for writing this library was to create and understand
how a Windows service works. I ended up thinking "how difficult could it be
to support other operating systems?" Well, it was not easy. This library can
control the state of a daemon (start, stop, install, and uninstall) as well
as turn your application code into a daemon.

## Supported systems
- Linux
    - systemd
    - System V (init.d) (if systemd is unavailable)
- macOS
    - launchd
- Windows
    - Windows service

## API
This library provides three primary interfaces for creating and managing
a daemon:
- Controller
- Daemonizer
- Application

The Controller is used to control the state of a daemon. Implementations
communicate with the operating system's daemon management software to
query a daemon's status, start or stop it, and install and uninstall it.
A Controller is configured using the ControllerConfig struct. This struct
provides the necessary information about a daemon (such as its ID).
It also provides customization options, such as the start up type.

Daemonizer is used to turn your application into a daemon. Implementations
of this interface use operating system specific calls and logic to properly
run your code as a daemon. This is facilitated by the Application interface.
Usage of a Controller is not required when using a Daemonizer. You may
implement your own daemon management tooling while leveraging the Daemonizer
to run your application. Please review the "Gotchas" documentation in the
Daemonizer interface if you choose to use your own management tooling.

The Application interface is used by the Daemonizer to run your application
code as a daemon. Implement this interface in your application and use the
Daemonizer to run your program.

#### Example daemon
The [examples/filewriter](examples/filewriter/main.go) provides a basic example
of an application that uses the Controller to control its state, and the
Daemonizer interface to daemonize the application.

## Design philosophies
I made a few design decisions along the way that may seem non-obvious. This
section will explain my thoughts on these decisions.

#### Why do I need to implement an interface?
One of the most prominent decisions is requiring users to implement the
`Application` interface. The reasoning behind this is mainly due to operating
system limitations (System V daemons need to fork exec, for example).
In addition, I did not want to risk log output (e.g., `log.Println()`)
occurring before the daemon can connect the logger to the operating system's
logging tool. Even if the OS just collects stderr - there could be a race
between the daemon starting and the application code running.

#### Why are there so many interfaces?
The use of interfaces is necessitated by the sheer number of operating systems.
Separating the concerns of "how do I control the daemon?", and "how do I turn
my code into a daemon?" seemed logical. I did not want to mix these two
responsibilities. Also, many people consider it weird for a daemon to change
its own state.

## Inspirations
Special thanks and acknowledgement must be made for the following individuals.
Their work was both inspirational and helpful for problem-solving ideas while
working on this project:
- [Igor "takama" Dolzhikov](https://github.com/takama) for his awesome work
on his [daemon](https://github.com/takama/daemon) project
- [Daniel "kardianos" Theophanes](https://github.com/kardianos) for his excellent
work on his [service](https://github.com/kardianos/service) project
- The [OpenSSH project](https://github.com/openssh/openssh-portable)
contributors and maintainers for their sshd init.d script, which I based this
project's init.d script on

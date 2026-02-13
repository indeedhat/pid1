package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	errExecFailedTpl = "failed to exec '%s' with error: %w"

	usage = `PID1 - init for containers

This is a minimal init implementiation for use in containers.
It allows you to execute a program under a valid init process.

Usage:
    pid1 [options] <COMMAND> [args]

Options:
    -h, -help
        Show help message
    -adopt
        Adopt child processes after the main process exits (default true)
`
)

type Options struct {
	Adopt bool
}

var (
	opts = Options{}
)

func main() {
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
	}
	flag.BoolVar(&opts.Adopt, "adopt", true, "Adopt child processes after the main process exits")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "no command provided")
		os.Exit(1)
	}

	if err := initSubReaper(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh)

	childPid, err := spawnChild(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to spawn child process")
		os.Exit(1)
	}

	go forwardSignals(childPid, sigCh)
	exitCode := waitAndReap(childPid)

	os.Exit(exitCode)
}

func forwardSignals(pid int, ch <-chan os.Signal) {
	for sig := range ch {
		err := syscall.Kill(-pid, sig.(syscall.Signal))
		if err != nil && err != syscall.ESRCH {
			fmt.Fprintf(os.Stderr, "failed to forward signal %v: %v\n", sig, err)
		}
	}
}

func spawnChild(argv []string) (int, error) {
	// spawn command in its own process group
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		if errno, ok := err.(*os.PathError); ok {
			switch errno.Err {
			case syscall.ENOENT:
				return 127, fmt.Errorf(errExecFailedTpl, argv[0], err)
			case syscall.EACCES:
				return 127, fmt.Errorf(errExecFailedTpl, argv[0], err)
			}
		}

		return 1, fmt.Errorf(errExecFailedTpl, argv[0], err)
	}

	return cmd.Process.Pid, nil
}

func initSubReaper() error {
	if err := enableSubReaper(); err != nil {
		return errors.New("failed to start subreaper process")
	}

	return nil
}

func enableSubReaper() error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
}

func waitAndReap(mainPid int) int {
	var (
		exitCode   int
		mainExited bool
	)

	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, 0, nil)

		if err != nil {
			if err == syscall.ECHILD {
				break
			}
			continue
		}

		if pid != mainPid {
			continue
		}

		mainExited = true

		if status.Exited() {
			exitCode = status.ExitStatus()
		} else if status.Signaled() {
			exitCode = 128 + int(status.Signal())
		}

		if !opts.Adopt {
			syscall.Kill(-mainPid, syscall.SIGTERM)
		}
	}

	if !mainExited {
		return 1
	}

	return exitCode
}

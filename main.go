package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/davecgh/go-spew/spew"
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
    -orphan-policy
        Set the policy for handling orphan processes [adopt, kill] (default adopt)

Environment Variables:
    PID1_ADITIONAL_SERVICES
        path to a icl config file specifying aditional services to run along side the main process
`
)

const (
	orphanAdopt = "adopt"
	orphanKill  = "kill"
)

type options struct {
	OrphanPolicy string
}

var (
	opts = options{}
)

type pid1Config struct {
	sigCh  chan os.Signal
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh)

	code, err := run(os.Args[1:], &pid1Config{
		sigCh:  sigCh,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	})
	if err != nil {
		fmt.Fprint(os.Stderr, err)
	}

	os.Exit(code)
}

func run(args []string, p1Conf *pid1Config) (int, error) {
	args, err := parseOpts(args)
	if err != nil {
		return 1, err
	}

	svcConf, err := loadConfig()
	if err != nil {
		return 1, fmt.Errorf("Failed to load config: %s", err)
	}

	if err := initSubReaper(); err != nil {
		return 1, err
	}

	childPid, err := spawnChild(args, p1Conf)
	if err != nil {
		// NB: this is a bit hacky, in the case of an error childPid is actually the error code
		return childPid, fmt.Errorf("failed to spawn child process: %s", err)
	}

	if err := bootAditionalServices(childPid, p1Conf, svcConf); err != nil {
		return 1, fmt.Errorf("Failed to boot aditional services: %s", err)
	}

	go forwardSignals(childPid, p1Conf.sigCh)

	return waitAndReap(childPid), nil
}

func parseOpts(args []string) ([]string, error) {
	fs := flag.NewFlagSet("pid1", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(flag.CommandLine.Output(), usage) }
	fs.StringVar(&opts.OrphanPolicy, "orphan-policy", orphanAdopt, "Adopt child processes after the main process exits")
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		return nil, errors.New("no command provided")
	}

	if opts.OrphanPolicy != orphanAdopt && opts.OrphanPolicy != orphanKill {
		return nil, errors.New("invalid orphan policy")
	}

	return fs.Args(), nil
}

type prefixWriter struct {
	prefix string
	w      io.Writer
}

func (p *prefixWriter) Write(data []byte) (int, error) {
	if p.w == nil {
		return -1, errors.New("prefixWriter incorrectly setup")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		_, err := p.w.Write([]byte("[" + p.prefix + "] " + line + "\n"))
		if err != nil {
			return 0, err
		}
	}

	return len(data), scanner.Err()
}

func bootAditionalServices(mainPid int, p1Conf *pid1Config, svcConf *AditionalServices) error {
	if svcConf == nil {
		spew.Dump("no config")
		return nil
	}

	for _, svc := range svcConf.Services {
		spew.Dump("booting " + svc.Name)

		if svc.Critical && svc.AutoRestart {
			_ = syscall.Kill(mainPid, syscall.SIGTERM)
			_ = syscall.Kill(-mainPid, syscall.SIGTERM)
			spew.Dump("bad critical")
			return fmt.Errorf("critical service '%s' cannot have an auto_restart policy", svc.Name)
		}

		if !isExecutable(svc.Command) {
			_ = syscall.Kill(mainPid, syscall.SIGTERM)
			_ = syscall.Kill(-mainPid, syscall.SIGTERM)
			spew.Dump("not executable")
			return fmt.Errorf("cannot find an executable command for svc %s", svc.Name)
		}

		go func() {
			for {
				// I am specifically not doing CommandContext here as i want the process to all get reaped manually
				// by waitAndReap, these all get added to the same process group
				cmd := exec.Command(svc.Command, svc.Args...)
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Setpgid: true,
					Pgid:    mainPid,
				}

				if svc.CaptureOutput {
					spew.Dump("capturing output for " + svc.Name)
					if svc.CapturePrefix {
						spew.Dump("with prefix " + svc.Name)
						cmd.Stdout = &prefixWriter{svc.Name, p1Conf.stdout}
						cmd.Stderr = &prefixWriter{svc.Name, p1Conf.stderr}
					} else {
						cmd.Stdout = p1Conf.stdout
						cmd.Stderr = p1Conf.stderr
					}
				}

				spew.Dump("cmd err ", cmd.Run())

				if svc.Critical {
					spew.Dump("critical" + svc.Name)
					_ = syscall.Kill(mainPid, syscall.SIGTERM)
					_ = syscall.Kill(-mainPid, syscall.SIGTERM)
					return
				}

				if !svc.AutoRestart {
					spew.Dump("no restart" + svc.Name)
					return
				}
			}
		}()
	}

	return nil
}

func forwardSignals(pid int, ch <-chan os.Signal) {
	for sig := range ch {
		err := syscall.Kill(-pid, sig.(syscall.Signal))
		if err != nil && err != syscall.ESRCH {
			fmt.Fprintf(os.Stderr, "failed to forward signal %v: %v\n", sig, err)
		}
	}
}

func spawnChild(argv []string, p1Conf *pid1Config) (int, error) {
	// spawn command in its own process group
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	cmd.Stdin = p1Conf.stdin
	cmd.Stdout = p1Conf.stdout
	cmd.Stderr = p1Conf.stderr

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
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return errors.New("failed to start subreaper process")
	}

	return nil
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

		if opts.OrphanPolicy == orphanKill {
			syscall.Kill(-mainPid, syscall.SIGTERM)
		}
	}

	if !mainExited {
		return 1
	}

	return exitCode
}

func isExecutable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	if err == nil {
		return true
	}

	stat, err := os.Stat(cmd)
	if err != nil {
		return false
	}

	mode := stat.Mode()

	return !mode.IsDir() && mode&0111 != 0
}

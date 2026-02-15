package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	scripts, _ := filepath.Glob("./fixtures/*")
	for _, s := range scripts {
		_ = os.Chmod(s, 0755)
	}
	_ = os.Chmod("./fixtures/not_executable.sh", 0644)

	os.Exit(m.Run())
}

var runTestCases = []struct {
	name          string
	args          []string
	expectedCode  int
	expectedError error
}{
	{
		name:         "exit code",
		args:         []string{"sh", "./fixtures/exit_code.sh"},
		expectedCode: 32,
	},
	{
		name:         "not executable",
		args:         []string{"./fixtures/not_executable.sh"},
		expectedCode: 127,
		expectedError: errors.New(
			"failed to spawn child process: failed to exec './fixtures/not_executable.sh' with error: " +
				"fork/exec ./fixtures/not_executable.sh: permission denied",
		),
	},
	{
		name:         "not exists",
		args:         []string{"./fixtures/not_exists.sh"},
		expectedCode: 127,
		expectedError: errors.New(
			"failed to spawn child process: failed to exec './fixtures/not_exists.sh' with error: fork/exec " +
				"./fixtures/not_exists.sh: no such file or directory",
		),
	},
}

func TestRun(t *testing.T) {
	for _, testCase := range runTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			conf := defaultP1TestConf()
			code, err := run(testCase.args, conf)

			require.Equal(t, testCase.expectedError, err)
			require.Equal(t, testCase.expectedCode, code)
		})
	}
}

type runResponse struct {
	code int
	err  error
}

func TestRunSigtermFormard(t *testing.T) {
	conf := defaultP1TestConf()
	done := make(chan runResponse, 1)

	go func() {
		code, err := run([]string{"./fixtures/signal_forward.sh"}, conf)
		done <- runResponse{code, err}
	}()

	time.Sleep(500 * time.Millisecond)

	conf.sigCh <- syscall.SIGTERM

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		require.Equal(t, 0, resp.code)
	case <-time.After(3 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}

func TestRunOrphanReaping(t *testing.T) {
	conf := defaultP1TestConf()
	done := make(chan runResponse, 1)

	go func() {
		code, err := run([]string{"-orphan-policy", "kill", "./fixtures/orphan.sh"}, conf)
		done <- runResponse{code, err}
	}()

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		require.Equal(t, 0, resp.code)

		running, err := isProcRunning("sleep 10")
		require.Nil(t, err)
		require.False(t, running, "sleep 10 is still running")

	case <-time.After(5 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}

func isProcRunning(cmd string) (bool, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid := entry.Name()
		if _, err := strconv.Atoi(pid); err != nil {
			continue
		}

		cmdlinePath := filepath.Join("/proc", pid, "cmdline")
		data, err := os.ReadFile(cmdlinePath)
		if err != nil {
			continue
		}

		cmdline := bytes.ReplaceAll(data, []byte{0}, []byte{' '})

		if bytes.Contains(cmdline, []byte(cmd)) {
			return true, nil
		}
	}

	return false, nil
}

func defaultP1TestConf() *pid1Config {
	stdout := []byte{}
	stderr := []byte{}

	return &pid1Config{
		sigCh:  make(chan os.Signal, 1),
		stdin:  bytes.NewReader(nil),
		stdout: bytes.NewBuffer(stdout),
		stderr: bytes.NewBuffer(stderr),
	}
}

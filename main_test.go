package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
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
			sigCh := make(chan os.Signal, 1)
			code, err := run(testCase.args, sigCh)

			spew.Dump(code, err)
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
	sigCh := make(chan os.Signal, 1)
	done := make(chan runResponse, 1)

	go func() {
		code, err := run([]string{"./fixtures/signal_forward.sh"}, sigCh)
		done <- runResponse{code, err}
	}()

	time.Sleep(500 * time.Millisecond)

	sigCh <- syscall.SIGTERM

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		require.Equal(t, 0, resp.code)
	case <-time.After(3 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}

func TestRunOrphanReaping(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	done := make(chan runResponse, 1)

	go func() {
		code, err := run([]string{"-orphan-policy", "kill", "./fixtures/orphan.sh"}, sigCh)
		done <- runResponse{code, err}
	}()

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		require.Equal(t, 0, resp.code)

		ps := exec.Command("ps", "-ef")
		out, err := ps.Output()
		if err != nil {
			t.Fatal(err)
		}
		require.NotContains(t, "sleep 10", string(out))

	case <-time.After(5 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}
